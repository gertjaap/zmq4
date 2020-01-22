// Copyright 2018 The go-zeromq Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zmq4_test

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

var (
	reqreps = []testCaseReqRep{
		{
			name:     "tcp-req-rep",
			endpoint: must(EndPoint("tcp")),
			req:      zmq4.NewReq(bkg),
			req2:     zmq4.NewReq(bkg),
			rep:      zmq4.NewRep(bkg),
		}, /*
			{
				name:     "ipc-req-rep",
				endpoint: "ipc://ipc-req-rep",
				req:      zmq4.NewReq(bkg),
				req2:     zmq4.NewReq(bkg),
				rep:      zmq4.NewRep(bkg),
			},
			{
				name:     "inproc-req-rep",
				endpoint: "inproc://inproc-req-rep",
				req:      zmq4.NewReq(bkg),
				req2:     zmq4.NewReq(bkg),
				rep:      zmq4.NewRep(bkg),
			},*/
	}
)

type testCaseReqRep struct {
	name     string
	skip     bool
	endpoint string
	req      zmq4.Socket
	req2     zmq4.Socket
	rep      zmq4.Socket
}

func TestReqRep(t *testing.T) {
	var (
		reqName  = zmq4.NewMsgString("NAME")
		reqLang  = zmq4.NewMsgString("LANG")
		reqQuit  = zmq4.NewMsgString("QUIT")
		reqName2 = zmq4.NewMsgString("NAME2")
		reqLang2 = zmq4.NewMsgString("LANG2")
		reqQuit2 = zmq4.NewMsgString("QUIT2")
		repName  = zmq4.NewMsgString("zmq4")
		repLang  = zmq4.NewMsgString("Go")
		repQuit  = zmq4.NewMsgString("bye")
		repName2 = zmq4.NewMsgString("zmq42")
		repLang2 = zmq4.NewMsgString("Go2")
		repQuit2 = zmq4.NewMsgString("bye2")
	)

	for i := range reqreps {
		tc := reqreps[i]
		t.Run(tc.name, func(t *testing.T) {
			log.Printf("Running test %s", tc.name)
			defer tc.req.Close()
			defer tc.req2.Close()
			defer tc.rep.Close()

			if tc.skip {
				t.Skipf(tc.name)
			}
			t.Parallel()

			ep := tc.endpoint
			cleanUp(ep)

			ctx, timeout := context.WithTimeout(context.Background(), 20*time.Second)
			defer timeout()

			grp, ctx := errgroup.WithContext(ctx)
			grp.Go(func() error {
				log.Printf("Listening on %s", ep)

				err := tc.rep.Listen(ep)
				if err != nil {
					return xerrors.Errorf("could not listen: %w", err)
				}

				if addr := tc.rep.Addr(); addr == nil {
					return xerrors.Errorf("listener with nil Addr")
				}

				loop1, loop2 := true, true
				for loop1 && loop2 {
					msg, err := tc.rep.Recv()
					if err != nil {
						return xerrors.Errorf("could not recv REQ message: %w", err)
					}
					var rep zmq4.Msg
					log.Printf("Received %s", string(msg.Frames[0]))
					switch string(msg.Frames[0]) {
					case "NAME":
						rep = repName
					case "LANG":
						rep = repLang
					case "QUIT":
						rep = repQuit
						loop1 = false
					case "NAME2":
						rep = repName2
					case "LANG2":
						rep = repLang2
					case "QUIT2":
						rep = repQuit2
						loop2 = false
					}
					log.Printf("Sending %s", rep)

					err = tc.rep.Send(rep)
					if err != nil {
						return xerrors.Errorf("could not send REP message to %v: %w", msg, err)
					}
				}
				log.Printf("Exited the REP loop")
				return err
			})
			grp.Go(func() error {

				err := tc.req2.Dial(ep)
				if err != nil {
					return xerrors.Errorf("could not dial: %w", err)
				}

				if addr := tc.req2.Addr(); addr != nil {
					return xerrors.Errorf("dialer with non-nil Addr")
				}

				for _, msg := range []struct {
					req zmq4.Msg
					rep zmq4.Msg
				}{
					{reqName2, repName2},
					{reqLang2, repLang2},
					{reqQuit2, repQuit2},
				} {
					err = tc.req2.Send(msg.req)
					if err != nil {
						return xerrors.Errorf("could not send REQ message %v: %w", msg.req, err)
					}
					rep, err := tc.req2.Recv()
					if err != nil {
						return xerrors.Errorf("could not recv REP message %v: %w", msg.req, err)
					}

					if got, want := rep, msg.rep; !reflect.DeepEqual(got, want) {
						log.Printf("[2] Got incorrect reply: %v vs %v", got, want)
						return xerrors.Errorf("got = %v, want= %v", got, want)
					} else {
						log.Printf("[2] Got correct reply: %v vs %v", got, want)
					}
				}
				log.Printf("Exited the REQ2 loop")
				return err
			})
			grp.Go(func() error {

				err := tc.req.Dial(ep)
				if err != nil {
					return xerrors.Errorf("could not dial: %w", err)
				}

				if addr := tc.req.Addr(); addr != nil {
					return xerrors.Errorf("dialer with non-nil Addr")
				}

				for _, msg := range []struct {
					req zmq4.Msg
					rep zmq4.Msg
				}{
					{reqName, repName},
					{reqLang, repLang},
					{reqQuit, repQuit},
				} {
					err = tc.req.Send(msg.req)
					if err != nil {
						return xerrors.Errorf("could not send REQ message %v: %w", msg.req, err)
					}
					rep, err := tc.req.Recv()
					if err != nil {
						return xerrors.Errorf("could not recv REP message %v: %w", msg.req, err)
					}

					if got, want := rep, msg.rep; !reflect.DeepEqual(got, want) {
						log.Printf("[1] Got incorrect reply: %v vs %v", got, want)
						return xerrors.Errorf("got = %v, want= %v", got, want)
					} else {
						log.Printf("[1] Got correct reply: %v vs %v", got, want)
					}
				}
				log.Printf("Exited the REQ loop")
				return err
			})
			if err := grp.Wait(); err != nil {
				t.Fatalf("error: %+v", err)
			}
		})
	}
}
