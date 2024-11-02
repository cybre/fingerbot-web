package packet

import (
	"bytes"
	"fmt"
	"log/slog"

	"github.com/cybre/fingerbot-web/internal/logging"
)

type Assembler struct {
	incoming        chan []byte
	assembled       chan []byte
	done            chan struct{}
	logger          *slog.Logger
	protocolVersion byte
	expectedLength  int
	inputBuffer     bytes.Buffer
	expectedPacket  int
}

func NewAssemmbler(logger *slog.Logger) *Assembler {
	pa := &Assembler{
		incoming:  make(chan []byte, 100),
		assembled: make(chan []byte, 100),
		done:      make(chan struct{}),
		logger:    logger,
	}
	go pa.run()
	return pa
}

func (a *Assembler) run() {
	for {
		select {
		case data := <-a.incoming:
			a.processData(data)
		case <-a.done:
			return
		}
	}
}

func (a *Assembler) processData(data []byte) {
	pos := 0
	packetNum, newPos, err := unpackInt(data, pos)
	if err != nil {
		a.logger.Error("Failed to unpack packet number", logging.ErrAttr(err))
		a.resetState()
		return
	}
	pos = newPos

	if packetNum == 0 {
		// Start of a new message
		if len(data) < pos+2 {
			a.logger.Warn("Data too short to contain totalLength")
			a.resetState()
			return
		}

		totalLength, newPos, err := unpackInt(data, pos)
		if err != nil {
			a.logger.Error("Failed to unpack total length", logging.ErrAttr(err))
			a.resetState()
			return
		}
		pos = newPos

		if pos >= len(data) {
			a.logger.Warn("Data too short to contain protocol version")
			a.resetState()
			return
		}

		a.protocolVersion = data[pos] >> 4
		pos++

		a.expectedLength = totalLength
		a.inputBuffer.Reset()
		a.expectedPacket = 1 // Next expected packet number

		a.logger.Debug("New notification",
			slog.Any("protocol_version", a.protocolVersion),
			slog.Any("total_length", a.expectedLength),
		)
	} else {
		if packetNum != a.expectedPacket {
			a.logger.Warn("Unexpected packet number",
				slog.Int("packet_num", packetNum),
				slog.Int("expected_packet", a.expectedPacket),
			)
			a.resetState()
			return
		}
		a.expectedPacket++
	}

	a.inputBuffer.Write(data[pos:])
	a.logger.Debug("Received packet part",
		slog.Int("packet_num", packetNum),
		slog.Int("packet_length", len(data[pos:])),
		slog.Int("buffer_length", a.inputBuffer.Len()),
		slog.Int("expected_length", a.expectedLength),
	)

	if a.inputBuffer.Len() >= a.expectedLength {
		assembledData := a.inputBuffer.Bytes()[:a.expectedLength]
		a.assembled <- assembledData
		a.resetState()
	}
}

func (a *Assembler) Incoming() chan<- []byte {
	return a.incoming
}

func (a *Assembler) Assembled() <-chan []byte {
	return a.assembled
}

func (a *Assembler) Stop() {
	close(a.done)
}

func (a *Assembler) resetState() {
	a.protocolVersion = 0
	a.expectedLength = 0
	a.inputBuffer.Reset()
	a.expectedPacket = 0
}

// unpackInt unpacks an integer using variable-length encoding
func unpackInt(data []byte, startPos int) (int, int, error) {
	result := 0
	offset := 0
	for offset < 5 {
		pos := startPos + offset
		if pos >= len(data) {
			return 0, 0, fmt.Errorf("data format error")
		}
		currByte := data[pos]
		result |= int(currByte&0x7F) << (offset * 7)
		offset++
		if (currByte & 0x80) == 0 {
			break
		}
	}
	if offset > 4 {
		return 0, 0, fmt.Errorf("data format error: integer too long")
	}

	return result, startPos + offset, nil
}
