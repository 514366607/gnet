// Copyright 2019 Andy Pan. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build darwin netbsd freebsd openbsd dragonfly linux

package gnet

import (
	"github.com/panjf2000/gnet/ringbuffer"
	"golang.org/x/sys/unix"
)

//type socket struct {
//	fd   int
//	conn *conn
//}

func (svr *server) activateMainReactor() {
	defer func() {
		svr.signalShutdown()
		svr.wg.Done()
	}()

	_ = svr.mainLoop.poller.Polling(func(fd int, note interface{}) error {
		if fd == 0 {
			return svr.mainLoop.loopNote(note)
		}

		if svr.ln.pconn != nil {
			return svr.mainLoop.loopUDPRead(fd)
		}
		nfd, sa, err := unix.Accept(fd)
		if err != nil {
			if err == unix.EAGAIN {
				return nil
			}
			return err
		}
		if err := unix.SetNonblock(nfd, true); err != nil {
			return err
		}
		lp := svr.eventLoopGroup.next()
		conn := &conn{
			fd:     nfd,
			sa:     sa,
			loop:   lp,
			inBuf:  ringbuffer.New(connRingBufferSize),
			outBuf: ringbuffer.New(connRingBufferSize),
		}
		_ = lp.loopOpened(conn)
		_ = lp.poller.Trigger(conn)
		//_ = lp.poller.Trigger(&socket{fd: nfd, conn: conn})
		return nil
	})
}

func (svr *server) activateSubReactor(loop *loop) {
	defer func() {
		svr.signalShutdown()
		svr.wg.Done()
	}()

	if loop.idx == 0 && svr.events.Tick != nil {
		go loop.loopTicker()
	}

	_ = loop.poller.Polling(func(fd int, note interface{}) error {
		if fd == 0 {
			return loop.loopNote(note)
		}
		conn := loop.connections[fd]
		if conn.outBuf.Length() > 0 {
			return loop.loopWrite(conn)
		}
		return loop.loopRead(conn)
	})
}