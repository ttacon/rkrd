package main

import (
	"fmt"
	"net"

	"github.com/alicebob/miniredis"
)

type Rkrd interface {
	Start() error
	HandleConnection() error
}

type rkrd struct {
	addr string

	listener  net.Listener
	miniredis *miniredis.Miniredis
}

func NewRkrd(addr string) Rkrd {
	return &rkrd{
		addr: addr,
	}
}

func (r *rkrd) Start() error {
	ln, err := net.Listen("tcp", r.addr)
	if err != nil {
		return err
	}
	r.listener = ln

	s, err := miniredis.Run()
	if err != nil {
		return err
	}
	r.miniredis = s

	return nil
}

func (r *rkrd) HandleConnection() error {
	conn, err := r.listener.Accept()
	if err != nil {
		return err
	}

	rkrdr := NewRkrdr(conn, r.miniredis)
	if err := rkrdr.Run(); err != nil {
		return err
	}

	return nil
}

type RecordInfo struct {
	Ctr  uint64
	Addr string
	Dir  string
	Msg  string
}

func (r *RecordInfo) String() string {
	return fmt.Sprintf(
		"ctr=%d dir=%q msg=%q",
		r.Ctr,
		r.Dir,
		r.Msg,
	)
}
