package libp2psatnet

import (
	loggerv2 "chainmaker.org/chainmaker/logger/v2"
	"chainmaker.org/chainmaker/protocol/v2"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

func TestSatNetConn(t *testing.T) {
	var (
		chain1 = "chain1"
		msgFlg = "TEST_MSG_SEND"

		node01Addr = "/ip4/127.0.0.1/tcp/11301"
		node02Addr = "/ip4/127.0.0.1/tcp/11302"

		privKey01 = "../testdata/private01.key"
		privKey02 = "../testdata/private02.key"
	)

	// new logger
	logger := loggerv2.GetLogger(loggerv2.MODULE_NET)

	log.Println("========================node01 make")
	// create a startup flag
	node01ReadySignalC := make(chan struct{})
	node01, err := makeNet(
		logger,
		WithReadySignalC(node01ReadySignalC),
		WithListenAddr(node01Addr),
		WithPrivKeyCrypto(privKey01),
	)
	if err != nil {
		log.Fatalln(err)
	}

	// node01 start
	log.Println("=>node01 start")
	err = node01.Start()
	if err != nil {
		log.Fatalln("node01 start failed: ", err)
	}
	node01Pid := node01.GetNodeUid()
	node01FullAddr := node01Addr + "/p2p/" + node01Pid
	fmt.Printf("node01 [%v] start\n", node01FullAddr)
	close(node01ReadySignalC)

	log.Println("===========================node02 make")
	node02ReadySignalC := make(chan struct{})

	node02, err := makeNet(
		logger,
		WithReadySignalC(node02ReadySignalC),
		WithListenAddr(node02Addr),
		WithPrivKeyCrypto(privKey02),
		WithSeeds(node01FullAddr),
	)
	log.Println("=>node02 start")
	err = node02.Start()
	if err != nil {
		log.Fatalln("node02 start err, ", err)
	}
	node02Pid := node02.GetNodeUid()
	node02FullAddr := node02Addr + "/p2p/" + node02Pid
	log.Printf("node02 [%v] start\n", node02FullAddr)

	close(node02ReadySignalC)

	// msg handler, used to process incoming topic information
	// the essence is a stream handler

	recvChan := make(chan bool)
	node2MsgHandler := func(peerId string, msg []byte) error {
		log.Printf("[node02][%v] recv a msg from peer[%v], msg:%v", chain1, peerId, string(msg))
		recvChan <- true
		return nil
	}

	err = node02.DirectMsgHandle(chain1, msgFlg, node2MsgHandler)
	if err != nil {
		log.Fatalln("node02 register msg handler err, ", err)
	}

	go func() {
		log.Printf("node01 send msg to node02 in %v\n", chain1)
		for {
			err = node01.SendMsg(chain1, node02Pid, msgFlg, []byte("hello i am node01"))
			if err != nil {
				log.Printf("node01 send msg to node02 in %v failed. err: %v", chain1, err)
				time.Sleep(time.Second)
				continue
			}
			break
		}
	}()

	select {
	case <-recvChan:
		log.Println("node01 send msg to node02 pass")
	}

	time.Sleep(time.Second * 10)
	fmt.Println("========start stop node==========")
	err = node01.Stop()
	if err != nil {
		log.Fatalln("node01 stop err", err)
	}

	err = node02.Stop()
	if err != nil {
		log.Fatalln("node02 stop err", err)
	}
}

type NetOption func(ln *LibP2pNet) error

func makeNet(logger protocol.Logger, opts ...NetOption) (*LibP2pNet, error) {
	localNet, err := NewLibP2pNet(logger)
	if err != nil {
		return nil, err
	}

	if err := apply(localNet, opts...); err != nil {
		return nil, err
	}
	return localNet, nil
}

// WithReadySignalC set a ready flag
func WithReadySignalC(signalC chan struct{}) NetOption {
	return func(ln *LibP2pNet) error {
		ln.Prepare().SetReadySignalC(signalC)
		return nil
	}
}

func WithListenAddr(addr string) NetOption {
	return func(ln *LibP2pNet) error {
		ln.Prepare().SetListenAddr(addr)
		return nil
	}
}

func WithPrivKeyCrypto(keyFile string) NetOption {
	return func(ln *LibP2pNet) error {
		var (
			err          error
			privKeyBytes []byte
		)
		privKeyBytes, err = os.ReadFile(keyFile)
		if err != nil {
			return err
		}
		ln.Prepare().SetKey(privKeyBytes)
		return nil
	}
}

func WithSeeds(seeds ...string) NetOption {
	return func(ln *LibP2pNet) error {
		for _, seed := range seeds {
			ln.Prepare().AddBootstrapsPeer(seed)
		}
		return nil
	}
}

// apply options.
func apply(ln *LibP2pNet, opts ...NetOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(ln); err != nil {
			return err
		}
	}
	return nil
}
