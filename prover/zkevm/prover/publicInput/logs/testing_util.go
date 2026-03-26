package logs

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	eth "github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// FirstTopicL2l1 is a helper function that outputs the limbs of the expected first topic of an L2L1 log
func FirstTopicL2l1() [common.NbLimbU256]field.Element {
	var firstTopic [common.NbLimbU256]field.Element
	firstTopicBytes := bridge.L2L1Topic0() // fixed expected value for the topic on the first topic row
	for i := range firstTopic {
		firstTopic[i].SetBytes(firstTopicBytes[i*2 : (i+1)*2])
	}
	return firstTopic
}

// FirstTopicRolling is a helper function that outputs the limbs of the expected first topic of a RollingHash log
func FirstTopicRolling() [common.NbLimbU256]field.Element {
	var firstTopicRolling [common.NbLimbU256]field.Element
	firstTopicRollingBytes := bridge.GetRollingHashUpdateTopic0() // fixed expected value for the topic on the first topic row
	for i := range firstTopicRolling {
		firstTopicRolling[i].SetBytes(firstTopicRollingBytes[i*2 : (i+1)*2])
	}
	return firstTopicRolling
}

// GenerateTopicsAndAddresses is a common test function that outputs sample topics and addresses
func GenerateTopicsAndAddresses() ([][common.NbLimbU256]field.Element, [][common.NbLimbEthAddress]field.Element) {
	var (
		topicsHi = []uint64{11, 21, 31, 41, 51, 61}
		topicsLo = []uint64{91, 101, 111, 121, 131, 141}
		address  = []eth.Address{
			types.DummyAddress(54),
			types.DummyAddress(64),
			types.DummyAddress(74),
			types.DummyAddress(84),
			types.DummyAddress(94),
		}
	)

	// convert the test addresses into Hi and Lo parts
	addressRaw := make([][common.NbLimbEthAddress]field.Element, len(address))
	for i := 0; i < len(address); i++ {
		addressRaw[i] = ConvertAddress(address[i])
	}

	// put hi and low parts of the topics at correct postitions in the limbs
	topicsRaw := make([][common.NbLimbU256]field.Element, len(topicsHi))
	for i := 0; i < len(topicsHi); i++ {
		topicsRaw[i] = newU256WithHiLoParts(topicsHi[i], topicsLo[i])
	}

	return topicsRaw, addressRaw
}

// GenerateSimpleL2L1Test generates test log info for testing the fetcher of message data from L2L1 logs
func GenerateSimpleL2L1Test() ([]LogInfo, types.EthAddress, string) {
	// get sample topics and addresses
	topics, address := GenerateTopicsAndAddresses()

	// fixed expected value for the topic on the first topic row
	firstTopic := FirstTopicL2l1()

	// compute a dummy bridge address for testing
	bridgeAddress := types.DummyAddress(2)
	bridgeAddr := ConvertAddress(bridgeAddress)
	var testLogs = [...]LogInfo{
		{
			LogType:  2, // log with 2 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(2),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{topics[0], topics[1]},
		},
		{
			LogType:  4, // an L2L1 log of the type we are interested in
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopic, topics[3], topics[4], topics[5]},
		},
		{
			LogType: MISSING_LOG, // missing log
			Topics:  nil,
		},
	}
	return testLogs[:], bridgeAddress, "GenerateSimpleL2L1Test"
}

// GenerateSimpleRollingTest generates test log info for testing the fetcher of message data from RollingHash logs
func GenerateSimpleRollingTest() ([]LogInfo, types.EthAddress, string) {
	// get sample topics and addresses
	topics, address := GenerateTopicsAndAddresses()

	// fixed expected value for the topic on the first topic row
	firstTopicRolling := FirstTopicRolling()

	// compute a dummy bridge address for testing
	bridgeAddress := types.DummyAddress(2)
	bridgeAddr := ConvertAddress(bridgeAddress)
	var testLogs = [...]LogInfo{
		{
			LogType:  3, // RollingHash log with 3 topics
			DataSize: newU128WithLoPart(3),
			noTopics: newU128WithLoPart(3),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicRolling, topics[2], topics[3]},
		},
		{
			LogType:  2, // log with 2 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(2),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{topics[0], topics[1]},
		},
	}
	return testLogs[:], bridgeAddress, "GenerateSimpleRollingTest"
}

// GenerateTestWithoutRelevantLogs generates tests that contain no relevant L2L1/RollingHash logs
func GenerateTestWithoutRelevantLogs() ([]LogInfo, types.EthAddress, string) {
	// get sample topics and addresses
	topics, address := GenerateTopicsAndAddresses()

	// fixed expected value for the topic on the first topic row
	firstTopic := FirstTopicL2l1()

	// compute a dummy bridge address for testing
	bridgeAddress := types.DummyAddress(2)
	bridgeAddr := ConvertAddress(bridgeAddress)
	var testLogs = [...]LogInfo{
		{
			LogType:  2, // log with 2 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(2),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{topics[0], topics[1]},
		},
		{
			LogType:  0, // log with 0 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(21),
			Address:  newU160WithHiLoParts(121, 333),
			Topics:   nil,
		},
		{
			LogType:  3, // log with 3 topics
			DataSize: newU128WithLoPart(3),
			noTopics: newU128WithLoPart(3),
			Address:  address[1],
			Topics:   [][common.NbLimbU256]field.Element{topics[1], topics[2], topics[3]},
		},
		{
			LogType:  4, // log of type 4 which has a wrong bridge data and will be skipped
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{firstTopic, topics[3], topics[4], topics[5]},
		},
		{
			LogType:  3, // log with 3 topics, but not a RollingHash log (wrong address)
			DataSize: newU128WithLoPart(191),
			noTopics: newU128WithLoPart(191),
			Address:  address[3],
			Topics:   [][common.NbLimbU256]field.Element{topics[1], topics[4], topics[5]},
		},
		{
			LogType:  4, // log of type 4 which has a wrong first topic in TopicsHi/TopicsLo
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{topics[3], topics[3], topics[4], topics[5]},
		},
		{
			LogType: MISSING_LOG, // missing log
			Topics:  nil,
		},
	}
	return testLogs[:], bridgeAddress, "GenerateTestWithoutRelevantLogs"
}

// GenerateTestData generates test log info for testing the fetcher of message data from L2L1/RollingHash logs
func GenerateLargeTest() ([]LogInfo, types.EthAddress, string) {
	// get sample topics and addresses
	topics, address := GenerateTopicsAndAddresses()

	// fixed expected value for the topic on the first topic row for L2L1/RollingHash logs
	firstTopicL2L1 := FirstTopicL2l1()
	firstTopicRolling := FirstTopicRolling()

	// compute a dummy bridge address for testing
	bridgeAddress := types.DummyAddress(2)
	bridgeAddr := ConvertAddress(bridgeAddress)
	var testLogs = [...]LogInfo{
		{
			LogType:  2, // log with 2 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(2),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{topics[0], topics[1]},
		},
		{
			LogType:  0, // log with 0 topics
			DataSize: newU128WithLoPart(21),
			noTopics: newU128WithLoPart(21),
			Address:  newU160WithHiLoParts(121, 333), // dummy address
			Topics:   nil,
		},
		{
			LogType:  3, // RollingHash log with 3 topics
			DataSize: newU128WithLoPart(3),
			noTopics: newU128WithLoPart(3),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicRolling, topics[2], topics[3]},
		},
		{
			LogType:  4, // an L2L1 log of the type we are interested in
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicL2L1, topics[3], topics[4], topics[5]},
		},
		{
			LogType:  4, // log of type 4 which has a wrong bridge data and will be skipped
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  address[0],
			Topics:   [][common.NbLimbU256]field.Element{firstTopicL2L1, topics[3], topics[4], topics[5]},
		},
		{
			LogType:  3, // log with 3 topics, but not a RollingHash log
			DataSize: newU128WithLoPart(191),
			noTopics: newU128WithLoPart(191),
			Address:  address[3],
			Topics:   [][common.NbLimbU256]field.Element{topics[1], topics[4], topics[5]},
		},
		{
			LogType:  4, // an L2L1 log of the type we are interested in
			DataSize: newU128WithLoPart(34443),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicL2L1, topics[2], topics[3], topics[4]},
		},
		{
			LogType:  4, // log of type 4 which has a wrong first topic in TopicsHi/TopicsLo
			DataSize: newU128WithLoPart(213),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{topics[3], topics[3], topics[4], topics[5]},
		},
		{
			LogType:  4, // an L2L1 log of the type we are interested in
			DataSize: newU128WithLoPart(100),
			noTopics: newU128WithLoPart(4),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicL2L1, topics[0], topics[4], topics[5]},
		},
		{
			LogType: MISSING_LOG, // missing log
			Topics:  nil,
		},
		{
			LogType:  3, // RollingHash log with 3 topics
			DataSize: newU128WithLoPart(312),
			noTopics: newU128WithLoPart(3),
			Address:  bridgeAddr,
			Topics:   [][common.NbLimbU256]field.Element{firstTopicRolling, topics[0], topics[5]},
		},
	}
	return testLogs[:], bridgeAddress, "GenerateLargeTest"
}

func newU128WithLoPart(lo uint64) [common.NbLimbU128]field.Element {
	var res [common.NbLimbU128]field.Element
	res[common.NbLimbU128-1].SetUint64(lo)
	return res
}

func newU160WithHiLoParts(hi, lo uint64) [common.NbLimbEthAddress]field.Element {
	var res [common.NbLimbEthAddress]field.Element
	res[common.NbLimbEthAddress-common.NbLimbU128-1].SetUint64(hi)
	res[common.NbLimbEthAddress-1].SetUint64(lo)
	return res
}

func newU256WithHiLoParts(hi, lo uint64) [common.NbLimbU256]field.Element {
	var res [common.NbLimbU256]field.Element
	res[common.NbLimbU128-1].SetUint64(hi)
	res[common.NbLimbU256-1].SetUint64(lo)
	return res
}
