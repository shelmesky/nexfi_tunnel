package main

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	encoder *gob.Encoder
	decoder *gob.Decoder

	wg sync.WaitGroup
)

type ClientData struct {
	DestAddr string
	Data     []byte
}

const (
	cIFF_TUN    = 0x0001
	cIFF_TAP    = 0x0002
	cIFF_NO_PI  = 0x1000
	BUFFERSIZE  = 4096
	SERVER_ADDR = "0.0.0.0:9199"
)

func main() {
	log.Println("Start server")

	server_sock, err := net.Listen("tcp", SERVER_ADDR)

	if err != nil {
		log.Println("can not listen on tcp:", err)
		return
	}

	log.Println("Server listen:", SERVER_ADDR)

	for {
		conn, err := server_sock.Accept()
		if err != nil {
			continue
		}

		encoder = gob.NewEncoder(conn)
		decoder = gob.NewDecoder(conn)

		iface, err := NewTAP("")

		go ConnToIface(conn, iface)
		go IfaceToConn(conn, iface)
		wg.Add(2)
	}

	wg.Wait()
}

func ConnToIface(conn net.Conn, iface *Interface) {
	defer wg.Done()
	client_data := new(ClientData)
	for {
		err := decoder.Decode(client_data)
		if err == io.EOF {
			log.Println("Client connection close")
			conn.Close()
			break
		}

		if err != nil {
			log.Println("decode data failed:", err)
			conn.Close()
			break
		}

		_, err = iface.Write(client_data.Data)

		if err != nil {
			log.Println("Write data to NIC failed:", err)
			conn.Close()
			iface.Close()
		}
	}
}

func IfaceToConn(conn net.Conn, iface *Interface) {
	defer wg.Done()
	buffer := make([]byte, BUFFERSIZE)
	client_data := new(ClientData)
	for {
		n, err := iface.Read(buffer)
		log.Printf("read %d bytes data from NIC\n", n)
		if err != nil {
			log.Println("Read from NIC failed:", err)
			iface.Close()
			conn.Close()
			break
		}

		client_data.DestAddr = "client"
		client_data.Data = buffer[:n]

		err = encoder.Encode(client_data)
		if err != nil {
			log.Println("Send data to client failed:", err)
			iface.Close()
			conn.Close()
			break
		}
	}
}

type ifReq struct {
	Name  [0x10]byte
	Flags uint16
	pad   [0x28 - 0x10 - 2]byte
}

func newTAP(ifName string) (ifce *Interface, err error) {
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	name, err := createInterface(file.Fd(), ifName, cIFF_TAP|cIFF_NO_PI)
	if err != nil {
		return nil, err
	}
	ifce = &Interface{isTAP: true, ReadWriteCloser: file, name: name}
	return
}

func newTUN(ifName string) (ifce *Interface, err error) {
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	name, err := createInterface(file.Fd(), ifName, cIFF_TUN|cIFF_NO_PI)
	if err != nil {
		return nil, err
	}
	ifce = &Interface{isTAP: false, ReadWriteCloser: file, name: name}
	return
}

func createInterface(fd uintptr, ifName string, flags uint16) (createdIFName string, err error) {
	var req ifReq
	req.Flags = flags
	copy(req.Name[:], ifName)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TUNSETIFF), uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		err = errno
		return
	}
	createdIFName = strings.Trim(string(req.Name[:]), "\x00")
	return
}

// Interface is a TUN/TAP interface.
type Interface struct {
	isTAP bool
	io.ReadWriteCloser
	name string
}

// Create a new TAP interface whose name is ifName.
// If ifName is empty, a default name (tap0, tap1, ... ) will be assigned.
// ifName should not exceed 16 bytes.
func NewTAP(ifName string) (ifce *Interface, err error) {
	return newTAP(ifName)
}

// Create a new TUN interface whose name is ifName.
// If ifName is empty, a default name (tap0, tap1, ... ) will be assigned.
// ifName should not exceed 16 bytes.
func NewTUN(ifName string) (ifce *Interface, err error) {
	return newTUN(ifName)
}

// Returns true if ifce is a TUN interface, otherwise returns false;
func (ifce *Interface) IsTUN() bool {
	return !ifce.isTAP
}

// Returns true if ifce is a TAP interface, otherwise returns false;
func (ifce *Interface) IsTAP() bool {
	return ifce.isTAP
}

// Returns the interface name of ifce, e.g. tun0, tap1, etc..
func (ifce *Interface) Name() string {
	return ifce.name
}
