package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/utils/types"
	"golang.org/x/time/rate"
)

var (
	_ http.Handler = &passToChanHandler{}
)

// todo @gusiri: TestShomeiReceptor needs updated test data (miniTraceFile) for the
// koala bear field transition. The hardcoded JSON hex values (e.g. zkParentStateRootHash,
// proof siblings) contain BLS-field elements that are too large for koalabear encoding.
// Each 4-byte limb must be < koalabear modulus (~2^31-1). Regenerate miniTraceFile
// with valid koalabear-encoded octuplet values.
func TestShomeiReceptor(t *testing.T) {
	t.Skip("test data needs to be regenerated with valid koalabear-encoded values")

	retChan := make(chan []byte, 1)
	// #nosec G114 -- since it's for a test, we don't care if the server does not
	// have a timeout.
	go http.ListenAndServe(":8080", &passToChanHandler{ret: retChan})

	sclient := shomeiClient{
		hostport:   "http://localhost:8080",
		client:     http.DefaultClient,
		maxRetries: 10,
		throttler: rate.NewLimiter(
			rate.Every(20*time.Millisecond),
			throttlerBucketSize,
		),
	}

	resp, err := sclient.fetchStateTransitionProofs(
		context.Background(),
		&zkEVMStateMerkleProofV0Req{
			StartBlockNumber: 0,
			EndBlockNumber:   1,
			ShomeiVersion:    "0.0.1-dev-18823579",
		},
	)

	if err != nil {
		t.Fatalf("could not get the response : %s", err)
	}

	// Wait for the request to come back to us from the channel.
	<-retChan

	_, errs := inspectTrace(types.KoalaOctuplet{}, resp, true)

	if len(errs) != 0 {
		t.Fatalf("got invalid traces : %s", errors.Join(errs...))
	}

}

type passToChanHandler struct {
	ret chan []byte
}

func (h *passToChanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("server got the request\n")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	h.ret <- body
	fmt.Printf("server sent in the chan\n")
	w.Write([]byte(miniTraceFile))
	fmt.Printf("server wrote back the response\n")
}

var miniTraceFile string = `{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
      "zkParentStateRootHash": "0x04c3c6de7195a187bc89fb4f8b68e93c7d675f1eed585b00d0e1e6241a321f86",
      "zkStateMerkleProof": [
          [
              {
                  "location":"0x",
                  "nextFreeNode":3,
                  "subRoot":"0x0e963ac1c981840721b20ccd7f5f2392697a8c9e1211dc67397a4a02e36ac23e",
                  "leaf":{
                     "hkey":"0x0b9887ed089160e457c4078941214f313dacfb71a8ed1818da3468ef1fdbe282",
                     "hval":"0x11314cf80cdd63a376e468ea9e6c672109bcfe516f0349382df82e1a876ca8b2",
                     "prevLeaf":0,
                     "nextLeaf":1
                  },
                  "proof":{
                     "leafIndex":2,
                     "siblings":[
                        "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "0x0e9a73b45ee2ff08c4431dc6e1f5266e05fc18bdf63b5a567f33b1bcfa3c6a69",
                        "0x03d32149dded57ddc5b1de7982c50ddebd44938118a1a7a77c05d0cd3893e7af",
                        "0x09827c8482e0ad5f566e0f3937a04a99eb8298c5fd10d812cb95cf53144488d2",
                        "0x106fa28252bc7a4d5e84940fcd0b9120f59c371f9977c6e42384bed365d908b9",
                        "0x0188ae0e9b728197d8ce998ac605b16f87d5c815690918525f3f19c6d10e0659",
                        "0x09c64ccd7021b40f4578f1ee24de81f079f7b0aa6e8a52a1b0833c1d219f32a8",
                        "0x0df25a23a4aa91719cb5445e6b1944078f1cbdf2de3b12ab37d63fb9d7e89007",
                        "0x06644a89954a1e4c49903c218d78dd5b09419db3088f84c919c938a5f98eda17",
                        "0x0edd0129edd35191a183ecd28cbcab2a48ad381215d8544acf35248639835dcd",
                        "0x0b971345bfa43e192ca2fb1c9ddd19f2dddf461243b1a54fdd5a4d581f850c11",
                        "0x09ea86c5cd59ac4bfca4e46e7b50bb37c8327350888ba71112ecf3f5093baaef",
                        "0x10c439d656480d21a08c068717556fb8104a7a76e26f60e393ce4e36ae21e07b",
                        "0x08b60393196453ee74fdf240449d9aa2569875b43596ea2621eecda8d8909acd",
                        "0x0f3f9cf1e5ba6bdbb6daafc405bcceac97270fe89265b6a0faa2ba4bfd5cbf5d",
                        "0x0b03678742039acaae14fd3964e2d6261b74410043c536f07bcf1bc4495d9f84",
                        "0x0133209cd7936e208da6b743428ff7195e8ef92d3dac72472146ac7497355ed1",
                        "0x070382f72e9f322433fb44fc4acfefd74b277b19b6cc1784379e7ca7338a2978",
                        "0x02a9fd706c3c223f9374481b7495fb775c1675407556d93f1edabfe54b3fc9b2",
                        "0x1276c046afd611be02a66cf85498d7210a15293357afe07968a86c89356662f5",
                        "0x0e42718d49cb8c4be515181eda51f41d3b8198af5a2139a4670a8ee06b904a2b",
                        "0x0defe934a1ae079cf6ec6022145b60128eeb30503eea4404da990fc2b2430ea8",
                        "0x0b7a8a9fe0ee619c9bd7ff504dcb47bdce0193546b53a79dedd5251f4f56f36c",
                        "0x0b2ae68e3af633dac72090cc9c9b0dce76cebf5117101a265f54b3b9a851b3cd",
                        "0x004d50e626bda007887a31f60883e58bce50a1a3e7a3384b9ec18dab319dd458",
                        "0x079081f446c9a0c7b404834742cea1909426ccfc4696d19e1a08531b0cc30368",
                        "0x0969f4e85b86f0eb36ad13dfb1f35346d7d6518308dc27e73452c649850f1a89",
                        "0x1092d1b2349c4fbc88ea0202cf88685e4e316c99697063f786201b27d46e2c22",
                        "0x11c8aeb3dc3ca059a29ba20d4471b20987d74a0d79ff8ecda247df6a02eca554",
                        "0x014030b5cbe31660da2d33b6b1265b82bbde9a7ab7f331f8b274f2b798a45a3b",
                        "0x0cdf7d06a4b4b0e71713048f5f6ea86016467e909a27bfeeeca67b56c17e2739",
                        "0x0f5dc218160db17cfe8044d7ac4fd55dfcbdf2676815e2c15388f189bf144cd8",
                        "0x07f048ac696418580a55a864a10ed030871fd615d5ab460c54d6184c16441d48",
                        "0x11c8e229e3e2ae40a4959e036d500753aaedb52cda67d9caf60f0629f0b4f306",
                        "0x090d53176fd185da729d0d68e0c0e646ef148f15864685f4ba56be7b7cbb2484",
                        "0x01f35ef342eaa841ee4306d38f2a1adeafe8967d23c31fe1a379b9a69353da6d",
                        "0x0a06dc31ae8e893bca0a076decb8c0caa9036b5f394abf79d7956411eef32255",
                        "0x060f08aed06ffb90efc9705dc38d37a7000da1add99cef1b8a84b9e72e7c8b7b",
                        "0x008a47a2a53dd5183a2dc127c399a004e2a6c7e60f73e104d7d79e6a2bd7e809",
                        "0x09e70d042c8766d9609b6d8169e4e99c664be32a9a4d6461726723723118cfbe"
                     ]
                  },
                  "key":"0x2400000000000000000000000000000000000000",
                  "value":"0x0000000000000000000000000000000000000000000000000000000000000041000000000000000000000000000000000000000000000000000000000000034307977874126658098c066972282d4c85f230520af3847e297fe7524f976873e50134373b65f439c874734ff51ea349327c140cde2e47a933146e6f9f2ad8eb17c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a4700000000000000000000000000000000000000000000000000000000000000000",
                  "type":0
               }
          ]
      ]
  }
}`
