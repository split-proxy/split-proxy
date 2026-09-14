package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

type SOCKS5Request struct {
	CMD  byte
	ATYP byte
	Host string
	Port uint16
}

type SOCKS5UDPPacket struct {
	ATYP    byte
	Host    string
	Port    uint16
	Payload []byte
}

func containsByte(data []byte, value byte) bool {
	for _, b := range data {
		if b == value {
			return true
		}
	}

	return false
}

func readSOCKS5Request(reader *bufio.Reader) (*SOCKS5Request, error) {
	header := make([]byte, 4)

	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}

	if header[0] != 0x05 {
		return nil, fmt.Errorf("invalid SOCKS5 version")
	}

	req := &SOCKS5Request{
		CMD:  header[1],
		ATYP: header[3],
	}

	if header[2] != 0x00 {
		return nil, fmt.Errorf("invalid RSV")
	}

	switch req.ATYP {
	case 0x01: // IPv4
		buf := make([]byte, 4)

		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}

		req.Host = net.IP(buf).String()

	case 0x03: // DOMAIN
		length, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		buf := make([]byte, int(length))

		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}

		req.Host = string(buf)

	case 0x04: // IPv6
		buf := make([]byte, 16)

		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}

		req.Host = net.IP(buf).String()

	default:
		return nil, fmt.Errorf(
			"unsupported SOCKS5 address type: %d",
			req.ATYP,
		)
	}

	portBuf := make([]byte, 2)

	if _, err := io.ReadFull(reader, portBuf); err != nil {
		return nil, err
	}

	req.Port = binary.BigEndian.Uint16(portBuf)

	return req, nil
}

func writeSOCKS5Success(client net.Conn) error {
	_, err := client.Write([]byte{
		0x05,                   // VER
		0x00,                   // REP = succeeded
		0x00,                   // RSV
		0x01,                   // ATYP = IPv4
		0x00, 0x00, 0x00, 0x00, // BND.ADDR
		0x00, 0x00, // BND.PORT
	})

	return err
}

func handleSOCKS5(
	client net.Conn,
	reader *bufio.Reader,
	authStore *AuthStore,
) {
	remoteAddr := client.RemoteAddr().String()

	infof(
		"[SOCKS5] connection started remote=%s",
		remoteAddr,
	)

	header := make([]byte, 2)

	if _, err := io.ReadFull(reader, header); err != nil {
		warnf(
			"[SOCKS5] failed to read greeting remote=%s: %v",
			remoteAddr,
			err,
		)
		return
	}

	if header[0] != 0x05 {
		warnf(
			"[SOCKS5] invalid protocol version remote=%s version=%d",
			remoteAddr,
			header[0],
		)
		return
	}

	nMethods := int(header[1])

	debugf(
		"[SOCKS5] client methods received remote=%s count=%d",
		remoteAddr,
		nMethods,
	)

	methods := make([]byte, nMethods)

	if _, err := io.ReadFull(reader, methods); err != nil {
		warnf(
			"[SOCKS5] failed to read authentication methods remote=%s: %v",
			remoteAddr,
			err,
		)
		return
	}

	// Предпочитаем USERNAME/PASSWORD.
	if containsByte(methods, 0x02) {
		infof(
			"[SOCKS5] selecting username/password authentication remote=%s",
			remoteAddr,
		)

		if _, err := client.Write([]byte{
			0x05,
			0x02,
		}); err != nil {
			warnf(
				"[SOCKS5] failed to send authentication method remote=%s: %v",
				remoteAddr,
				err,
			)
			return
		}

		if err := authenticateSOCKS5(
			client,
			reader,
			authStore,
		); err != nil {
			warnf(
				"[SOCKS5] authentication failed remote=%s: %v",
				remoteAddr,
				err,
			)
			return
		}

		infof(
			"[SOCKS5] authentication successful remote=%s",
			remoteAddr,
		)

	} else {
		warnf(
			"[SOCKS5] no supported authentication method remote=%s",
			remoteAddr,
		)

		_, _ = client.Write([]byte{
			0x05,
			0xff,
		})

		return
	}

	// Теперь читаем request.
	req, err := readSOCKS5Request(reader)
	if err != nil {
		warnf(
			"[SOCKS5] request read failed remote=%s: %v",
			remoteAddr,
			err,
		)
		return
	}

	infof(
		"[SOCKS5] request received remote=%s cmd=%d atyp=%d host=%s port=%d",
		remoteAddr,
		req.CMD,
		req.ATYP,
		req.Host,
		req.Port,
	)

	switch req.CMD {
	case 0x01:
		infof(
			"[SOCKS5] CONNECT requested remote=%s host=%s port=%d",
			remoteAddr,
			req.Host,
			req.Port,
		)

		handleSOCKS5Connect(
			client,
			reader,
			req,
		)

		infof(
			"[SOCKS5] CONNECT handler finished remote=%s host=%s port=%d",
			remoteAddr,
			req.Host,
			req.Port,
		)

	case 0x03:
		infof(
			"[SOCKS5] UDP ASSOCIATE requested remote=%s",
			remoteAddr,
		)

		handleSOCKS5UDPAssociate(client)

		infof(
			"[SOCKS5] UDP ASSOCIATE handler finished remote=%s",
			remoteAddr,
		)

	default:
		warnf(
			"[SOCKS5] unsupported command remote=%s cmd=%d",
			remoteAddr,
			req.CMD,
		)

		// Command not supported.
		_, err := client.Write([]byte{
			0x05,
			0x07, // Command not supported
			0x00,
			0x01,
			0, 0, 0, 0,
			0, 0,
		})

		if err != nil {
			warnf(
				"[SOCKS5] failed to send unsupported command response remote=%s: %v",
				remoteAddr,
				err,
			)
		}
	}
}

func handleSOCKS5Connect(
	client net.Conn,
	reader *bufio.Reader,
	req *SOCKS5Request,
) {
	remoteAddr := client.RemoteAddr().String()

	address := net.JoinHostPort(
		req.Host,
		strconv.Itoa(int(req.Port)),
	)

	route := ResolveRoute(req.Host)

	infof(
		"[SOCKS5] CONNECT route selected remote=%s target=%s route=%s",
		remoteAddr,
		address,
		route,
	)

	if route == RouteDirect {
		infof(
			"[SOCKS5] CONNECT using direct route remote=%s target=%s",
			remoteAddr,
			address,
		)

		handleDirect(
				&BufferedConn{
					Conn:   client,
					reader: reader,
				},
				address,
				func() error {
					infof(
						"[SOCKS5] CONNECT established remote=%s target=%s",
						remoteAddr,
						address,
					)

					return writeSOCKS5Success(client)
				},
			)

		infof(
			"[SOCKS5] direct CONNECT finished remote=%s target=%s",
			remoteAddr,
			address,
		)

		return
	}

	infof(
		"[SOCKS5] CONNECT using broker route remote=%s target=%s route=%s",
		remoteAddr,
		address,
		route,
	)

	handleBroker(
		client,
		reader,
		"CONNECT",
		address,
		route,
		func() error {
			err := writeSOCKS5Success(client)

			if err != nil {
				warnf(
					"[SOCKS5] failed to send CONNECT success remote=%s target=%s: %v",
					remoteAddr,
					address,
					err,
				)
				return err
			}

			infof(
				"[SOCKS5] CONNECT established remote=%s target=%s route=%s",
				remoteAddr,
				address,
				route,
			)

			return nil
		},
	)

	infof(
		"[SOCKS5] broker CONNECT finished remote=%s target=%s route=%s",
		remoteAddr,
		address,
		route,
	)
}

func authenticateSOCKS5(
	client net.Conn,
	reader *bufio.Reader,
	authStore *AuthStore,
) error {

	// SOCKS5 Username/Password Authentication
	//
	// Request:
	//
	// +----+------+----------+------+----------+
	// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
	// +----+------+----------+------+----------+
	// | 1  |  1   | 1-255    |  1   | 1-255    |
	// +----+------+----------+------+----------+

	version, err := reader.ReadByte()
	if err != nil {
		return err
	}

	if version != 0x01 {
		return fmt.Errorf(
			"invalid SOCKS5 auth version: %d",
			version,
		)
	}

	usernameLen, err := reader.ReadByte()
	if err != nil {
		return err
	}

	usernameBytes := make([]byte, int(usernameLen))

	if _, err := io.ReadFull(
		reader,
		usernameBytes,
	); err != nil {
		return err
	}

	passwordLen, err := reader.ReadByte()
	if err != nil {
		return err
	}

	passwordBytes := make([]byte, int(passwordLen))

	if _, err := io.ReadFull(
		reader,
		passwordBytes,
	); err != nil {
		return err
	}

	username := string(usernameBytes)
	password := string(passwordBytes)

	if !authStore.CheckCredentials(
		username,
		password,
	) {
		_, _ = client.Write([]byte{
			0x01,
			0x01,
		})

		return fmt.Errorf(
			"invalid credentials",
		)
	}

	_, err = client.Write([]byte{
		0x01,
		0x00,
	})

	return err
}
