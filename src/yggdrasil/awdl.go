package yggdrasil

import (
	"errors"
	"io"
	"sync"
)

type awdl struct {
	core       *Core
	mutex      sync.RWMutex // protects interfaces below
	interfaces map[string]*awdlInterface
}

type awdlInterface struct {
	link   *linkInterface
	rwc    awdlReadWriteCloser
	peer   *peer
	stream stream
}

type awdlReadWriteCloser struct {
	fromAWDL chan []byte
	toAWDL   chan []byte
}

func (c awdlReadWriteCloser) Read(p []byte) (n int, err error) {
	select {
	case packet := <-c.fromAWDL:
		n = copy(p, packet)
		return n, nil
	default:
		return 0, io.EOF
	}
}

func (c awdlReadWriteCloser) Write(p []byte) (n int, err error) {
	c.toAWDL <- p
	return len(p), nil
}

func (c awdlReadWriteCloser) Close() error {
	close(c.fromAWDL)
	close(c.toAWDL)
	return nil
}

func (l *awdl) init(c *Core) error {
	l.core = c
	l.mutex.Lock()
	l.interfaces = make(map[string]*awdlInterface)
	l.mutex.Unlock()

	return nil
}

func (l *awdl) create(name, local, remote string) (*awdlInterface, error) {
	rwc := awdlReadWriteCloser{
		fromAWDL: make(chan []byte, 1),
		toAWDL:   make(chan []byte, 1),
	}
	s := stream{}
	s.init(rwc)
	link, err := l.core.link.create(&s, name, "awdl", local, remote)
	if err != nil {
		return nil, err
	}
	intf := awdlInterface{
		link: link,
		rwc:  rwc,
	}
	l.mutex.Lock()
	l.interfaces[name] = &intf
	l.mutex.Unlock()
	go intf.link.handler()
	return &intf, nil
}

func (l *awdl) getInterface(identity string) *awdlInterface {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	if intf, ok := l.interfaces[identity]; ok {
		return intf
	}
	return nil
}

func (l *awdl) shutdown(identity string) error {
	if intf, ok := l.interfaces[identity]; ok {
		close(intf.link.closed)
		intf.rwc.Close()
		l.mutex.Lock()
		delete(l.interfaces, identity)
		l.mutex.Unlock()
		return nil
	}
	return errors.New("Interface not found or already closed")
}
