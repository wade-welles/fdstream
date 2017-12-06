package fdstream

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

//TODO support longer mesages

//Max size of message
const (
	MaxMessageSize              = 1e5
	messageHeaderSize           = 9
	erMissRoutingCode      byte = 254
	erGeneralErrorCode     byte = 255
	erTimeoutCode          byte = 253
	erDuplicateIdErrorCode byte = 252
)

var (
	//ErrEmptyName is specify error used in name is mandatary value (Sync client)
	ErrEmptyName = errors.New("Empty name of message")
	//ErrTooShortMessage mean incomplete header
	ErrTooShortMessage = errors.New("Too short message")
	//ErrBinaryLength mean rest of bytes have incorrect length according header
	ErrBinaryLength = errors.New("Incorrect binary length")
)

//Message is a communication message for async and sync client
type Message struct {
	//Name of message, some metadata about payload
	// it is optional
	Name string
	//Id is optional for async but mandatary for sync communication to get correct responce by Id
	Id uint32
	//Code is an a flag of somebody, fill free to use flag < 200
	// Code with value more than 200 mean some error or problem
	Code byte
	//Payload is a user data to be send
	Payload []byte
}

var bufferPool = sync.Pool{}

func getBuf() *bytes.Buffer {
	if b := bufferPool.Get(); b != nil {
		buf := b.(*bytes.Buffer)
		buf.Reset()
		return buf
	}
	return bytes.NewBuffer(make([]byte, 0, MaxMessageSize))
}

func NewMessage(code byte, name string, payload []byte) *Message {
	return &Message{
		Code:    code,
		Name:    name,
		Payload: payload,
	}
}

//Marshal marshall message to byte array with simple structure [code,name length, value length, name,value]
func (m *Message) Marshal() ([]byte, error) {
	buf := getBuf()
	uintNamelen := uint16(len(m.Name))
	uintValueLen := uint16(len(m.Payload))

	header := make([]byte, messageHeaderSize, messageHeaderSize)
	header[0] = m.Code
	binary.BigEndian.PutUint32(header[1:5], m.Id)
	binary.BigEndian.PutUint16(header[5:7], uintNamelen)
	binary.BigEndian.PutUint16(header[7:9], uintValueLen)

	buf.Write(header)
	buf.WriteString(m.Name)
	buf.Write(m.Payload)
	res := buf.Bytes() //TODO maybe copy instread
	bufferPool.Put(buf)
	return res, nil
}

//WriteTo implements io.WriteTo interface to write directly to i0.Writer
// It will be slower than marshal and direct write marshaled bytes.
func (m *Message) WriteTo(writer io.Writer) (n int, err error) {
	buf := getBuf()
	uintNamelen := uint16(len(m.Name))
	uintValueLen := uint16(len(m.Payload))

	header := make([]byte, messageHeaderSize, messageHeaderSize)
	header[0] = m.Code
	binary.BigEndian.PutUint32(header[1:5], m.Id)
	binary.BigEndian.PutUint16(header[5:7], uintNamelen)
	binary.BigEndian.PutUint16(header[7:9], uintValueLen)

	buf.Write(header)
	buf.WriteString(m.Name)

	buf.Write(m.Payload)
	n, err = writer.Write(buf.Bytes())
	bufferPool.Put(buf)
	return
}

//unmarshal create message from specified byte array or return error
func unmarshal(b []byte) (*Message, error) {
	if len(b) < messageHeaderSize {
		return nil, ErrTooShortMessage
	}

	var (
		code               byte
		m                  *Message
		cursor, payloadLen uint16
		ID                 uint32
	)

	code, ID, cursor, payloadLen = unmarshalHeader(b)
	m = &Message{
		Code:    code,
		Id:      ID,
		Payload: make([]byte, payloadLen, payloadLen), //This line slowdown Unmarshal by 2-3 times
	}

	if len(b) != int(messageHeaderSize+cursor+payloadLen) {
		return nil, ErrBinaryLength
	}

	if cursor > 0 {
		m.Name = string(b[messageHeaderSize : messageHeaderSize+cursor])
	}
	cursor += messageHeaderSize

	if payloadLen > 0 {
		copy(m.Payload, b[cursor:cursor+payloadLen])
	}
	return m, nil

}

//unmarshalHeader is unsafe read expect at least 11 bytes length
func unmarshalHeader(b []byte) (code byte, id uint32, nameLen, payloadLen uint16) {
	code = b[0]
	id = binary.BigEndian.Uint32(b[1:5])
	nameLen = binary.BigEndian.Uint16(b[5:7])

	payloadLen = binary.BigEndian.Uint16(b[7:9])
	return
}

//Len calculate current length of message in bytes
func (m *Message) Len() int {
	return messageHeaderSize + len(m.Name) + len(m.Payload)
}
