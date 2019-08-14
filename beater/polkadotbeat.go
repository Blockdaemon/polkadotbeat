package beater

import (
	"fmt"
	"time"
	"net/http"
	"strings"
	"io/ioutil"
	"math"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/Blockdaemon/polkadotbeat/config"

	"gitlab.com/Blockdaemon/untyped"
	"gitlab.com/Blockdaemon/jsonrpc2"
)

// Polkadotbeat configuration.
type Polkadotbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

func CallJSONRPC(url string, message jsonrpc2.Message) (jsonrpc2.Response, error) {
	resp, err := http.Post(url, "application/json", strings.NewReader(message.String()))
	if err != nil {
		return jsonrpc2.Response{}, fmt.Errorf("error while requesting '%s': %s", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return jsonrpc2.Response{}, fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return jsonrpc2.Response{}, fmt.Errorf("error reading body: %s", err)
	}

	jsonRPCResp, err := jsonrpc2.ParseResponse(body)
	if err != nil {
		return jsonrpc2.Response{}, fmt.Errorf("error, response payload is not valid jsonrpc2: %s", body)
	}

	return jsonRPCResp, nil
}

// New creates an instance of polkadotbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Polkadotbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

// Run starts polkadotbeat.
func (bt *Polkadotbeat) Run(b *beat.Beat) error {
	logp.Info("polkadotbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}



	ticker := time.NewTicker(bt.config.Period)

	var blockNumber float64
	var peerCount float64
	var highestBlockNumber float64

	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
			/////////////////////////////////////////
			peersMessage := jsonrpc2.NewMessage("system_peers", nil, 1)
			url := "http://" + bt.config.PolkadotHost + ":" + bt.config.PolkadotPort

			peersResponse, err := CallJSONRPC(url, peersMessage)
			if err != nil {
				return err
			}
			peersList := peersResponse.Result.([]interface{})
			peerCount = 0
			highestBlockNumber = -1.0

			for _, peer := range peersList {
				peerCount++
				peerBlockCount, err := untyped.GetFloat64(peer, "bestNumber")
				if err != nil {
					return err
				}
				highestBlockNumber = math.Max(highestBlockNumber, peerBlockCount)
			}

			blockMessage := jsonrpc2.NewMessage("chain_getBlock", nil, 1)
			blockResponse, err := CallJSONRPC(url, blockMessage)
			if err != nil {
				return err
			}

			fmt.Println(blockResponse.Result)
			blockNumber, err = untyped.GetFloat64(blockResponse.Result, "block.number")
			if err != nil {
				return err
			}
			/////////////////////////////////////
		}

		event := beat.Event{
			Timestamp: time.Now(),
			Fields: common.MapStr{
				"type":    "ChainState",
				"blockNumber": int64(blockNumber),
				"peerCount":   int64(peerCount),
				"highestBlockNumber":   int64(highestBlockNumber),
			},
		}
		bt.client.Publish(event)
		logp.Info("Event sent")
	}
}

// Stop stops polkadotbeat.
func (bt *Polkadotbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}
