package minecraft

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

type GameProfile struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Properties []GameProfileProperty `json:"properties"`
}

type GameProfileProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature"`
}

type minecraftSessionService struct {
	client *http.Client
}

func (s *minecraftSessionService) hasJoinedServer(username, serverHash string) (*GameProfile, error) {
	endpoint := "https://sessionserver.mojang.com/session/minecraft/hasJoined?username=" + url.QueryEscape(username) + "&serverId=" + url.QueryEscape(serverHash)
	resp, err := s.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected status: " + resp.Status)
	}

	var profile GameProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
