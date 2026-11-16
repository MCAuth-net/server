package minecraft

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
)

type MinecraftServer struct {
	rsaKeyPair     *rsa.PrivateKey
	sessionService *minecraftSessionService
}

type PlayerHandler func(client *Client, profile GameProfile, disconnect func(string))

func NewMinecraftServer(handler PlayerHandler) {
	server := &MinecraftServer{}

	var err error
	server.rsaKeyPair, err = rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}

	server.sessionService = &minecraftSessionService{
		client: &http.Client{},
	}

	port := os.Getenv("PORT")
	fmt.Println("Bound on " + port)
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go newLoginAuthHandler(handler, server.rsaKeyPair, server.sessionService, conn).run()
	}
}

func kick(conn net.Conn, w *bufio.Writer, jsonChatComponent string) {
	out := &bytes.Buffer{}
	writeVarInt(out, 0x00)
	writeString(out, jsonChatComponent)

	frame := &bytes.Buffer{}
	writeVarInt(frame, out.Len())
	frame.Write(out.Bytes())

	w.Write(frame.Bytes())
	w.Flush()
	conn.Close()
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := 0
	numRead := 0
	for {
		read, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		length |= int(read&0x7F) << (7 * numRead)
		numRead++
		if numRead > 3 {
			return nil, errors.New("length prefix too long")
		}
		if read&0x80 == 0 {
			break
		}
	}
	frame := make([]byte, length)
	_, err := io.ReadFull(r, frame)
	return frame, err
}

func writeFrame(w *bufio.Writer, payload []byte) error {
	frame := &bytes.Buffer{}
	writeVarInt(frame, len(payload))
	frame.Write(payload)
	if _, err := w.Write(frame.Bytes()); err != nil {
		return err
	}
	return w.Flush()
}

type packetReader interface {
	io.Reader
	io.ByteReader
}

func readVarInt(r io.ByteReader) (int, error) {
	value := 0
	position := 0
	for {
		currentByte, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= int(currentByte&0x7F) << position
		if currentByte&0x80 == 0 {
			break
		}
		position += 7
		if position >= 32 {
			return 0, errors.New("VarInt too big")
		}
	}
	return value, nil
}

func writeVarInt(buf *bytes.Buffer, value int) {
	for (value & -128) != 0 {
		buf.WriteByte(byte(value&127) | 128)
		value = int(uint32(value) >> 7)
	}
	buf.WriteByte(byte(value))
}

func readString(r packetReader) (string, error) {
	length, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

func writeString(buf *bytes.Buffer, str string) {
	data := []byte(str)
	writeVarInt(buf, len(data))
	buf.Write(data)
}
