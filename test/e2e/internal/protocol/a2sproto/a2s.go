// Package a2sproto implements Valve's A2S (Source) query protocol client.
//
// Protocol reference: https://developer.valvesoftware.com/wiki/Server_queries
//
// Wire format (UDP connectionless):
//
//	0xFFFFFFFF (4-byte header) | type | payload...
//
// A2S_INFO request sends type 0x54 with an optional challenge.
// Modern servers (post-2020) often reply with S2C_CHALLENGE (0x41) to
// block reflection attacks; clients must resend the request with the
// 4-byte challenge appended.
//
// A2S_INFO response structure (after 0xFFFFFFFF 0x49 header):
//
//	1 byte protocol version
//	null-terminated string: server name
//	null-terminated string: map name
//	null-terminated string: folder/game directory
//	null-terminated string: game description
//	2 bytes uint16 LE: app ID
//	1 byte: players online
//	1 byte: max players
//	1 byte: bots
//	1 byte: server type ('d'=dedicated, 'l'=listen, 'p'=SourceTV proxy)
//	1 byte: environment ('l'=Linux, 'w'=Windows, 'm'=Mac)
//	1 byte: visibility (0=public, 1=private)
//	1 byte: VAC secured (0=no, 1=yes)
//	null-terminated string: game version
//
// A2S_PLAYER request sends type 0x55 with a 4-byte challenge (always required).
//
// A2S_PLAYER response structure (after 0xFFFFFFFF 0x44 header):
//
//	1 byte: player count
//	For each player:
//	  1 byte: index
//	  null-terminated string: player name
//	  4 bytes int32 LE: score (typically kills/points)
//	  4 bytes float32 LE: duration (seconds in session)
package a2sproto

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	headerFourCC      = 0xFFFFFFFF
	requestInfo       = 0x54 // 'T'
	requestPlayer     = 0x55 // 'U'
	responseChallenge = 0x41 // challenge response from server
	responseInfo      = 0x49 // 'I'
	responsePlayer    = 0x44 // 'D'
)

// Info describes a game server as reported by A2S_INFO.
type Info struct {
	Protocol    byte
	Name        string
	Map         string
	Folder      string
	Game        string
	ID          uint16
	Players     byte
	MaxPlayers  byte
	Bots        byte
	ServerType  byte
	Environment byte
	Visibility  byte
	VAC         byte
	Version     string
}

// Player describes one player on the server as reported by A2S_PLAYER.
type Player struct {
	Index    byte
	Name     string
	Score    int32
	Duration float32
}

// QueryInfo performs A2S_INFO, transparently handling the S2C_CHALLENGE flow.
// It dials addr (UDP), sends A2S_INFO, and if the server replies with a
// challenge, resends with the challenge appended.
func QueryInfo(ctx context.Context, addr string) (*Info, error) {
	// Use DialContext to let the kernel choose the local address based on the
	// destination's route. This avoids binding to loopback when the remote is
	// a non-loopback address (e.g., a Kubernetes ClusterIP), which would fail
	// with "sendto: invalid argument".
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("a2s: dial udp: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Set deadline on the socket itself.
	deadline := time.Now().Add(5 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	// First, try without a challenge.
	req := buildRequest(requestInfo, nil)

	if _, err := conn.Write(req); err != nil {
		return nil, fmt.Errorf("a2s: send info request: %w", err)
	}

	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, fmt.Errorf("a2s: receive info response: %w", err)
	}

	// Check if we got a challenge.
	if isChallenge(resp[:n]) {
		challenge, err := parseChallenge(resp[:n])
		if err != nil {
			return nil, fmt.Errorf("a2s: parse challenge: %w", err)
		}
		// Resend with the challenge appended.
		req = buildRequest(requestInfo, challenge)
		if _, err := conn.Write(req); err != nil {
			return nil, fmt.Errorf("a2s: send info request with challenge: %w", err)
		}
		_ = conn.SetDeadline(deadline)
		n, err = conn.Read(resp)
		if err != nil {
			return nil, fmt.Errorf("a2s: receive info response after challenge: %w", err)
		}
	}

	info, err := parseInfo(resp[:n])
	if err != nil {
		return nil, fmt.Errorf("a2s: parse info: %w", err)
	}
	return info, nil
}

// QueryPlayers performs A2S_PLAYER, obtaining a challenge via A2S_INFO first.
func QueryPlayers(ctx context.Context, addr string) ([]Player, error) {
	// First verify the server is reachable via A2S_INFO.
	_, err := QueryInfo(ctx, addr)
	if err != nil {
		return nil, err
	}

	// Use DialContext to let the kernel choose the local address based on the
	// destination's route. This avoids binding to loopback when the remote is
	// a non-loopback address (e.g., a Kubernetes ClusterIP), which would fail
	// with "sendto: invalid argument".
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("a2s: dial udp for players: %w", err)
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(5 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	// A2S_PLAYER always requires a challenge. Start with 0xFFFFFFFF sentinel.
	// If the server responds with S2C_CHALLENGE, echo the challenge back.
	challenge := []byte{0xFF, 0xFF, 0xFF, 0xFF}

	for attempt := 0; attempt < 2; attempt++ {
		req := buildRequest(requestPlayer, challenge)
		if _, err := conn.Write(req); err != nil {
			return nil, fmt.Errorf("a2s: send player request: %w", err)
		}

		resp := make([]byte, 8192)
		_ = conn.SetDeadline(deadline)
		n, err := conn.Read(resp)
		if err != nil {
			return nil, fmt.Errorf("a2s: receive player response: %w", err)
		}

		// Try to parse as player response. If it's a challenge, retry once.
		if isChallenge(resp[:n]) {
			if attempt == 0 {
				var err error
				challenge, err = parseChallenge(resp[:n])
				if err != nil {
					return nil, fmt.Errorf("a2s: parse player challenge: %w", err)
				}
				// Retry with the new challenge (loop continues).
				continue
			}
			// Already tried once with the challenge; give up.
			return nil, errors.New("a2s: server keeps sending challenges for players")
		}

		players, err := parsePlayers(resp[:n])
		if err != nil {
			return nil, fmt.Errorf("a2s: parse players: %w", err)
		}
		return players, nil
	}

	return nil, errors.New("a2s: failed to get players after retries")
}

// ---- wire helpers ----

func buildRequest(typ byte, challenge []byte) []byte {
	var buf bytes.Buffer
	// Write 0xFFFFFFFF header (LE)
	buf.WriteByte(0xFF)
	buf.WriteByte(0xFF)
	buf.WriteByte(0xFF)
	buf.WriteByte(0xFF)
	// Write request type
	buf.WriteByte(typ)
	// A2S_INFO requires the "Source Engine Query\0" magic string
	if typ == requestInfo {
		buf.WriteString("Source Engine Query\x00")
	}
	// Write challenge if provided
	if challenge != nil {
		buf.Write(challenge)
	}
	return buf.Bytes()
}

func isChallenge(resp []byte) bool {
	if len(resp) < 5 {
		return false
	}
	header := binary.LittleEndian.Uint32(resp[:4])
	return header == headerFourCC && resp[4] == responseChallenge
}

func parseChallenge(resp []byte) ([]byte, error) {
	if len(resp) < 9 {
		return nil, errors.New("challenge response too short")
	}
	return resp[5:9], nil
}

func parseInfo(resp []byte) (*Info, error) {
	if len(resp) < 6 {
		return nil, errors.New("info response too short")
	}

	header := binary.LittleEndian.Uint32(resp[:4])
	if header != headerFourCC {
		return nil, fmt.Errorf("invalid header: 0x%08x", header)
	}
	if resp[4] != responseInfo {
		return nil, fmt.Errorf("expected info response (0x49), got 0x%02x", resp[4])
	}

	r := bytes.NewReader(resp[5:])
	info := &Info{}

	var err error
	if info.Protocol, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read protocol: %w", err)
	}

	if info.Name, err = readCString(r); err != nil {
		return nil, fmt.Errorf("read name: %w", err)
	}
	if info.Map, err = readCString(r); err != nil {
		return nil, fmt.Errorf("read map: %w", err)
	}
	if info.Folder, err = readCString(r); err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}
	if info.Game, err = readCString(r); err != nil {
		return nil, fmt.Errorf("read game: %w", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &info.ID); err != nil {
		return nil, fmt.Errorf("read app id: %w", err)
	}

	if info.Players, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read players: %w", err)
	}
	if info.MaxPlayers, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read max players: %w", err)
	}
	if info.Bots, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read bots: %w", err)
	}
	if info.ServerType, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read server type: %w", err)
	}
	if info.Environment, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read environment: %w", err)
	}
	if info.Visibility, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read visibility: %w", err)
	}
	if info.VAC, err = r.ReadByte(); err != nil {
		return nil, fmt.Errorf("read vac: %w", err)
	}

	if info.Version, err = readCString(r); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}

	return info, nil
}

func parsePlayers(resp []byte) ([]Player, error) {
	if len(resp) < 6 {
		return nil, errors.New("player response too short")
	}

	header := binary.LittleEndian.Uint32(resp[:4])
	if header != headerFourCC {
		return nil, fmt.Errorf("invalid header: 0x%08x", header)
	}
	if resp[4] != responsePlayer {
		return nil, fmt.Errorf("expected player response (0x44), got 0x%02x", resp[4])
	}

	r := bytes.NewReader(resp[5:])
	count, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("read player count: %w", err)
	}

	players := make([]Player, 0, count)
	for i := byte(0); i < count; i++ {
		player := Player{}

		if player.Index, err = r.ReadByte(); err != nil {
			return nil, fmt.Errorf("read player %d index: %w", i, err)
		}

		if player.Name, err = readCString(r); err != nil {
			return nil, fmt.Errorf("read player %d name: %w", i, err)
		}

		if err := binary.Read(r, binary.LittleEndian, &player.Score); err != nil {
			return nil, fmt.Errorf("read player %d score: %w", i, err)
		}

		if err := binary.Read(r, binary.LittleEndian, &player.Duration); err != nil {
			return nil, fmt.Errorf("read player %d duration: %w", i, err)
		}

		players = append(players, player)
	}

	return players, nil
}

// readCString reads a null-terminated UTF-8 string.
func readCString(r io.Reader) (string, error) {
	var buf bytes.Buffer
	for {
		b := make([]byte, 1)
		if _, err := r.Read(b); err != nil {
			return "", err
		}
		if b[0] == 0 {
			break
		}
		buf.Write(b)
	}
	return buf.String(), nil
}
