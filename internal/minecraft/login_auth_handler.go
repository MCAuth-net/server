package minecraft

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
)

const (
	stateHandshake = iota
	stateStatus
	stateLoginStart
	stateEncryptionResponse
	stateClosed
)

type loginAuthHandler struct {
	handler        PlayerHandler
	rsaKeyPair     *rsa.PrivateKey
	sessionService *minecraftSessionService
	state          int
	client         *Client
	verifyToken    []byte
	conn           net.Conn
	reader         *bufio.Reader
	writer         *bufio.Writer
}

var Favicon = ""

func newLoginAuthHandler(handler PlayerHandler, rsaKeyPair *rsa.PrivateKey, sessionService *minecraftSessionService, conn net.Conn) *loginAuthHandler {
	return &loginAuthHandler{
		handler:        handler,
		rsaKeyPair:     rsaKeyPair,
		sessionService: sessionService,
		state:          stateHandshake,
		client: &Client{
			Conn: conn,
		},
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (h *loginAuthHandler) run() {
	defer h.conn.Close()
	for h.state != stateClosed {
		frame, err := readFrame(h.reader)
		if err != nil {
			return
		}
		in := bytes.NewReader(frame)
		packetId, err := readVarInt(in)
		if err != nil {
			return
		}
		if err := h.channelRead(packetId, in); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func (h *loginAuthHandler) close() {
	h.state = stateClosed
	h.conn.Close()
}

func (h *loginAuthHandler) channelRead(packetId int, in *bytes.Reader) error {
	switch h.state {
	case stateHandshake:
		if packetId != 0x00 {
			h.close()
			return nil
		}
		protocolVersion, err := readVarInt(in)
		if err != nil {
			return err
		}
		h.client.Protocol = protocolVersion

		serverAddress, err := readString(in)
		if err != nil {
			return err
		}
		h.client.ServerAddress = serverAddress

		var serverPort uint16
		if err := binary.Read(in, binary.BigEndian, &serverPort); err != nil {
			return err
		}
		nextState, err := readVarInt(in)
		if err != nil {
			return err
		}
		if nextState == 1 {
			h.state = stateStatus
		} else if nextState == 2 {
			h.state = stateLoginStart
		} else {
			h.close()
		}
		return nil

	case stateStatus:
		switch packetId {
		case 0x00:
			response := map[string]interface{}{
				"version": map[string]interface{}{
					"name":     "MCAuth",
					"protocol": h.client.Protocol,
				},
				"players": map[string]interface{}{
					"max":    1,
					"online": 0,
				},
				"description": map[string]string{
					"text": "MCAuth",
				},
			}

			if Favicon != "" {
				response["favicon"] = "data:image/png;base64," + Favicon
			}

			data, err := json.Marshal(response)
			if err != nil {
				return err
			}

			out := &bytes.Buffer{}
			writeVarInt(out, 0x00)
			writeString(out, string(data))

			return writeFrame(h.writer, out.Bytes())

		case 0x01:
			payload := make([]byte, 8)
			if _, err := io.ReadFull(in, payload); err != nil {
				return err
			}

			out := &bytes.Buffer{}
			writeVarInt(out, 0x01)
			out.Write(payload)

			if err := writeFrame(h.writer, out.Bytes()); err != nil {
				return err
			}

			h.state = stateClosed
			return nil

		default:
			h.close()
			return nil
		}

	case stateLoginStart:
		if packetId != 0x00 {
			h.close()
			return nil
		}
		claimedUsername, err := readString(in)
		if err != nil {
			return err
		}
		h.client.Username = claimedUsername

		h.verifyToken = make([]byte, 4)
		rand.Read(h.verifyToken)

		pubKeyBytes, err := x509.MarshalPKIXPublicKey(&h.rsaKeyPair.PublicKey)
		if err != nil {
			return err
		}

		out := &bytes.Buffer{}
		writeVarInt(out, 0x01)
		writeString(out, "")
		writeVarInt(out, len(pubKeyBytes))
		out.Write(pubKeyBytes)
		writeVarInt(out, len(h.verifyToken))
		out.Write(h.verifyToken)
		if h.client.Protocol >= 766 {
			out.WriteByte(1)
		}
		if err := writeFrame(h.writer, out.Bytes()); err != nil {
			return err
		}
		h.state = stateEncryptionResponse
		return nil

	case stateEncryptionResponse:
		if packetId != 0x01 {
			h.close()
			return nil
		}
		encryptedSecret, err := readByteArray(in)
		if err != nil {
			return err
		}
		encryptedToken, err := readByteArray(in)
		if err != nil {
			return err
		}

		sharedSecretBytes, err := rsa.DecryptPKCS1v15(rand.Reader, h.rsaKeyPair, encryptedSecret)
		if err != nil {
			return err
		}
		decryptedToken, err := rsa.DecryptPKCS1v15(rand.Reader, h.rsaKeyPair, encryptedToken)
		if err != nil {
			return err
		}

		if !bytes.Equal(h.verifyToken, decryptedToken) {
			h.close()
			return nil
		}

		if err := h.enableEncryption(sharedSecretBytes); err != nil {
			return err
		}

		pubKeyBytes, err := x509.MarshalPKIXPublicKey(&h.rsaKeyPair.PublicKey)
		if err != nil {
			return err
		}

		sha := sha1.New()
		sha.Write([]byte(""))
		sha.Write(sharedSecretBytes)
		sha.Write(pubKeyBytes)
		digest := sha.Sum(nil)

		serverHash := new(big.Int).SetBytes(digest)
		if digest[0]&0x80 != 0 {
			serverHash.Sub(serverHash, new(big.Int).Lsh(big.NewInt(1), uint(len(digest)*8)))
		}

		go func() {
			profile, err := h.sessionService.hasJoinedServer(h.client.Username, serverHash.Text(16))
			if err != nil {
				component, _ := json.Marshal(map[string]string{
					"text":  "Authentication servers unavailable.",
					"color": "red",
				})
				kick(h.conn, h.writer, string(component))
				return
			}
			if profile != nil {
				h.handler(h.client, *profile, func(reason string) {
					component, _ := json.Marshal(map[string]string{
						"text": reason,
					})
					kick(h.conn, h.writer, string(component))
				})
			} else {
				component, _ := json.Marshal(map[string]string{
					"text":  "Failed to verify username.",
					"color": "red",
				})
				kick(h.conn, h.writer, string(component))
			}
		}()
		return nil
	}
	return nil
}

func (h *loginAuthHandler) enableEncryption(sharedSecret []byte) error {
	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return err
	}

	h.reader = bufio.NewReader(cipher.StreamReader{R: h.conn, S: newCFB8(block, sharedSecret, true)})
	h.writer = bufio.NewWriter(cipher.StreamWriter{W: h.conn, S: newCFB8(block, sharedSecret, false)})
	return nil
}

type cfb8 struct {
	block     cipher.Block
	blockSize int
	in        []byte
	out       []byte
	decrypt   bool
}

func newCFB8(block cipher.Block, iv []byte, decrypt bool) *cfb8 {
	stream := &cfb8{
		block:     block,
		blockSize: block.BlockSize(),
		in:        make([]byte, block.BlockSize()),
		out:       make([]byte, block.BlockSize()),
		decrypt:   decrypt,
	}
	copy(stream.in, iv)
	return stream
}

func (s *cfb8) XORKeyStream(dst, src []byte) {
	for i := range src {
		s.block.Encrypt(s.out, s.in)
		b := src[i]
		if s.decrypt {
			s.in = append(s.in[1:], b)
		}
		dst[i] = b ^ s.out[0]
		if !s.decrypt {
			s.in = append(s.in[1:], dst[i])
		}
	}
}

func readByteArray(r packetReader) ([]byte, error) {
	length, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func escapeJson(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
