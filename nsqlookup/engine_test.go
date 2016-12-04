package nsqlookup

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

const (
	nodeTimeout = 1 * time.Minute
	tombTimeout = 10 * time.Millisecond
)

func testEngine(t *testing.T, do func(*testing.T, Engine)) {
	tests := []struct {
		Type string
		New  func() Engine
	}{
		{
			Type: "local",
			New: func() Engine {
				return NewLocalEngine(LocalConfig{
					NodeTimeout:      nodeTimeout,
					TombstoneTimeout: tombTimeout,
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Type, func(t *testing.T) {
			t.Parallel()

			e := test.New()
			defer e.Close()

			if info, err := e.LookupInfo(); err != nil {
				t.Error(err)
			} else if info.Type != test.Type {
				t.Error("bad engine type:", info.Type)
			}

			do(t, e)
		})
	}
}

func TestEngineClose(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		if err := e.Close(); err != nil {
			t.Error(err)
		}
	})
}

func TestEngineRegisterNode(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		for _, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				if err := e.RegisterNode(node); err != nil {
					t.Error(err)
				}
			})
		}

		t.Run("lookup-nodes", func(t *testing.T) {
			nodes2, err := e.LookupNodes()
			if err != nil {
				t.Error(err)
			}
			checkEqualNodes(t, nodes1, nodes2)
		})
	})
}

func TestEngineUnregisterNode(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		t.Run("unregister", func(t *testing.T) {
			if err := e.UnregisterNode(nodes1[0]); err != nil {
				t.Error(err)
			}
		})

		t.Run("lookup-nodes", func(t *testing.T) {
			nodes2, err := e.LookupNodes()
			if err != nil {
				t.Error(err)
			}
			checkEqualNodes(t, nodes1[1:], nodes2)
		})
	})
}

func TestEnginePingNode(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for _, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				if err := e.PingNode(node); err != nil {
					t.Error(err)
				}
			})
		}
	})
}

func TestEngineTombstoneTopic(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		topics1 := [][]string{
			[]string{"A"},
			[]string{"A", "B", "C"},
			nil,
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for i, node := range nodes1 {
			for _, topic := range topics1[i] {
				if err := e.RegisterTopic(node, topic); err != nil {
					t.Error(err)
				}
			}
		}

		t.Run("tombstone", func(t *testing.T) {
			for _, node := range nodes1 {
				t.Run(node.Hostname, func(t *testing.T) {
					if err := e.TombstoneTopic(node, "A"); err != nil {
						t.Error(err)
					}
				})
			}
		})

		for _, test := range []struct {
			topic string
			nodes []NodeInfo
		}{
			{"A", nil},
			{"B", []NodeInfo{nodes1[1]}},
			{"C", []NodeInfo{nodes1[1]}},
		} {
			t.Run(test.topic, func(t *testing.T) {
				nodes, err := e.LookupProducers(test.topic)
				if err != nil {
					t.Error(err)
				}
				checkEqualNodes(t, test.nodes, nodes)
			})
		}

		t.Run("lookup-topics", func(t *testing.T) {
			topics2, err := e.LookupTopics()
			if err != nil {
				t.Error(err)
			}
			checkEqualTopics(t, []string{"B", "C"}, topics2)
		})

		// Sleep for a little while to give time to the tombstone to expire.
		time.Sleep(2 * tombTimeout)

		for _, test := range []struct {
			topic string
			nodes []NodeInfo
		}{
			{"A", []NodeInfo{nodes1[0], nodes1[1]}},
			{"B", []NodeInfo{nodes1[1]}},
			{"C", []NodeInfo{nodes1[1]}},
		} {
			t.Run(test.topic, func(t *testing.T) {
				nodes, err := e.LookupProducers(test.topic)
				if err != nil {
					t.Error(err)
				}
				checkEqualNodes(t, test.nodes, nodes)
			})
		}

		t.Run("lookup-topics", func(t *testing.T) {
			topics2, err := e.LookupTopics()
			if err != nil {
				t.Error(err)
			}
			checkEqualTopics(t, []string{"A", "B", "C"}, topics2)
		})
	})
}

func TestEngineRegisterTopic(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		topics1 := [][]string{
			[]string{"A"},
			[]string{"A", "B", "C"},
			nil,
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for i, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				for _, topic := range topics1[i] {
					t.Run(topic, func(t *testing.T) {
						if err := e.RegisterTopic(node, topic); err != nil {
							t.Error(err)
						}
					})
				}
			})
		}

		for _, test := range []struct {
			topic string
			nodes []NodeInfo
		}{
			{"A", []NodeInfo{nodes1[0], nodes1[1]}},
			{"B", []NodeInfo{nodes1[1]}},
			{"C", []NodeInfo{nodes1[1]}},
			{"D", nil},
		} {
			t.Run(test.topic, func(t *testing.T) {
				nodes, err := e.LookupProducers(test.topic)
				if err != nil {
					t.Error(err)
				}
				checkEqualNodes(t, test.nodes, nodes)
			})
		}

		t.Run("lookup-topics", func(t *testing.T) {
			topics2, err := e.LookupTopics()
			if err != nil {
				t.Error(err)
			}
			checkEqualTopics(t, []string{"A", "B", "C"}, topics2)
		})
	})
}

func TestEngineUnregisterTopic(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		topics1 := [][]string{
			[]string{"A"},
			[]string{"A", "B", "C"},
			nil,
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for i, node := range nodes1 {
			for _, topic := range topics1[i] {
				if err := e.RegisterTopic(node, topic); err != nil {
					t.Error(err)
				}
			}
		}

		for _, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				if err := e.UnregisterTopic(node, "A"); err != nil {
					t.Error(err)
				}
			})
		}

		for _, test := range []struct {
			topic string
			nodes []NodeInfo
		}{
			{"A", nil},
			{"B", []NodeInfo{nodes1[1]}},
			{"C", []NodeInfo{nodes1[1]}},
			{"D", nil},
		} {
			t.Run(test.topic, func(t *testing.T) {
				nodes, err := e.LookupProducers(test.topic)
				if err != nil {
					t.Error(err)
				}
				checkEqualNodes(t, test.nodes, nodes)
			})
		}

		t.Run("lookup-topics", func(t *testing.T) {
			topics2, err := e.LookupTopics()
			if err != nil {
				t.Error(err)
			}
			checkEqualTopics(t, []string{"B", "C"}, topics2)
		})
	})
}

func TestEngineRegisterChannel(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		channels1 := [][]string{
			[]string{"1"},
			[]string{"1", "2", "3"},
			nil,
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for i, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				for _, channel := range channels1[i] {
					t.Run(channel, func(t *testing.T) {
						if err := e.RegisterChannel(node, "A", channel); err != nil {
							t.Error(err)
						}
					})
				}
			})
		}

		t.Run("lookup-channels", func(t *testing.T) {
			channels2, err := e.LookupChannels("A")
			if err != nil {
				t.Error(err)
			}
			checkEqualChannels(t, []string{"1", "2", "3"}, channels2)
		})
	})
}

func TestEngineUnregisterChannel(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		nodes1 := []NodeInfo{
			makeNodeInfo(1),
			makeNodeInfo(2),
			makeNodeInfo(3),
		}

		channels1 := [][]string{
			[]string{"1"},
			[]string{"1", "2", "3"},
			nil,
		}

		for _, node := range nodes1 {
			if err := e.RegisterNode(node); err != nil {
				t.Error(err)
			}
		}

		for i, node := range nodes1 {
			for _, channel := range channels1[i] {
				if err := e.RegisterChannel(node, "A", channel); err != nil {
					t.Error(err)
				}
			}
		}

		for _, node := range nodes1 {
			t.Run(node.Hostname, func(t *testing.T) {
				if err := e.UnregisterChannel(node, "A", "1"); err != nil {
					t.Error(err)
				}
			})
		}

		t.Run("lookup-channels", func(t *testing.T) {
			channels2, err := e.LookupChannels("A")
			if err != nil {
				t.Error(err)
			}
			checkEqualChannels(t, []string{"2", "3"}, channels2)
		})
	})
}

func TestEngineCheckHealth(t *testing.T) {
	testEngine(t, func(t *testing.T, e Engine) {
		if err := e.CheckHealth(); err != nil {
			t.Error(err)
		}
	})
}

func makeNodeInfo(i int) NodeInfo {
	return NodeInfo{
		RemoteAddress:    fmt.Sprintf("10.0.0.%d:35000", i),
		BroadcastAddress: fmt.Sprintf("10.0.0.%d", i),
		Hostname:         fmt.Sprintf("host-%d", i),
		TcpPort:          4150,
		HttpPort:         4151,
		Version:          "0.3.8",
	}
}

func checkEqualNodes(t *testing.T, n1 []NodeInfo, n2 []NodeInfo) {
	sortedNodes(n1)
	sortedNodes(n2)

	if !reflect.DeepEqual(n1, n2) {
		t.Error("bad nodes")
		t.Log("<<<", n1)
		t.Log(">>>", n2)
	}
}

func checkEqualTopics(t *testing.T, t1 []string, t2 []string) {
	sortedStrings(t1)
	sortedStrings(t2)

	if !reflect.DeepEqual(t1, t2) {
		t.Error("bad topics")
		t.Log("<<<", t1)
		t.Log(">>>", t2)
	}
}

func checkEqualChannels(t *testing.T, c1 []string, c2 []string) {
	sortedStrings(c1)
	sortedStrings(c2)

	if !reflect.DeepEqual(c1, c2) {
		t.Error("bad channels")
		t.Log("<<<", c1)
		t.Log(">>>", c2)
	}
}
