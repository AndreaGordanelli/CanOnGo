// Simple example
//  sck, err := canbus.New()
//  err = sck.Bind("vcan0")
//  for {
//      id, data, err := sck.Recv()
//  }
//
package Can

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

const frameSize = unsafe.Sizeof(frame{})

var (
	errorDataTooBig = errors.New("Frame Data is too big")
	errorIdTooBig   = errors.New("Frame ID is too big")
)

type device struct {
	fd int
}

// frame is a can_frame.
type frame struct {
	ID   [11]int16
	Len  byte
	_    [3]byte
	Data [8]byte
}

// Socket is a high-level representation of a CANBus socket.
type Socket struct {
	Interface *net.Interface
	Address   *unix.SockaddrCAN
	Device    device
}

func NewCan(port string) (*Socket, error) {
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW, unix.CAN_RAW)
	if err != nil {
		return nil, err
	}
	Iface, err := net.InterfaceByName(port)
	if err != nil {
		return nil, err
	}
	return &Socket{Iface, &unix.SockaddrCAN{Ifindex: Iface.Index}, device{fd}}, nil
}

// New returns a new CAN bus socket.
func New() (*Socket, error) {
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW, unix.CAN_RAW)
	if err != nil {
		return nil, err
	}

	return &Socket{Device: device{fd}}, nil
}

// Close closes the CAN bus socket.
func (sck *Socket) Close() error {
	return unix.Close(sck.Device.fd)
}

// Bind binds the socket on the CAN bus with the given address.
//
// Example:
//  err = sck.Bind("vcan0")
func (sck *Socket) Bind(Address string) error {
	Interface, err := net.InterfaceByName(Address)
	if err != nil {
		return err
	}

	sck.Interface = Interface
	sck.Address = &unix.SockaddrCAN{Ifindex: sck.Interface.Index}

	return unix.Bind(sck.Device.fd, sck.Address)
}

func (d device) Write(data []byte) (int, error) {
	return unix.Write(d.fd, data)
}

func (d device) Read(data []byte) (int, error) {
	return unix.Read(d.fd, data)
}

// iName returns the device name the socket is bound to.
func (sck *Socket) iName() string {
	if sck.Interface == nil {
		return "N/A"
	}
	return sck.Interface.Name
}

// Send sends data with a CAN_frame id to the CAN bus.
func (sck *Socket) Send(id uint32, data []byte) (int, error) {
	if len(data) > 8 {
		return 0, errorDataTooBig
	}

	id &= unix.CAN_SFF_MASK
	var frame [frameSize]byte
	binary.LittleEndian.PutUint32(frame[:4], id)
	frame[4] = byte(len(data))
	copy(frame[8:], data)

	return sck.Device.Write(frame[:])
}

// Recv receives data from the CAN socket.
// id is the CAN_frame id the data was originated from.
func (sck *Socket) Recv() (id uint32, data []byte, err error) {
	var frame [frameSize]byte
	n, err := io.ReadFull(sck.Device, frame[:])
	if err != nil {
		return id, data, err
	}

	if n != len(frame) {
		return id, data, io.ErrUnexpectedEOF
	}

	id = binary.LittleEndian.Uint32(frame[:4])
	id &= unix.CAN_SFF_MASK
	data = make([]byte, frame[4])
	copy(data, frame[8:])
	return id, data, nil
}
