package decoder

import (
	"fmt"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

type frameBuilderTest struct {
	message   string
	packets   []*rtp.Packet
	headBytes []byte
	samples   []*Frame
	maxLate   uint16
}

type fakeDepacketizer struct {
}

func (f *fakeDepacketizer) Unmarshal(r []byte) ([]byte, error) {
	return r, nil
}

type fakePartitionHeadChecker struct {
	headBytes []byte
}

func (f *fakePartitionHeadChecker) IsPartitionHead(payload []byte) bool {
	for _, b := range f.headBytes {
		if payload[0] == b {
			return true
		}
	}
	return false
}

func TestFrameBuilder(t *testing.T) {
	testData := []frameBuilderTest{
		{
			message: "FrameBuilder shouldn't emit a packet because we have a gap before a valid one",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			samples: []*Frame{},
			maxLate: 50,
		},
		{
			message: "FrameBuilder should emit a packet after a gap if PartitionHeadChecker assumes it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			headBytes: []byte{0x02},
			samples: []*Frame{
				{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 6},
			},
			maxLate: 50,
		},
		{
			message: "FrameBuilder shouldn't emit a packet after a gap if PartitionHeadChecker doesn't assume it head",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 5}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5007, Timestamp: 6}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5008, Timestamp: 7}, Payload: []byte{0x03}},
			},
			headBytes: []byte{},
			samples:   []*Frame{},
			maxLate:   50,
		},
		{
			message: "FrameBuilder should emit multiple valid packets",
			packets: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 1}, Payload: []byte{0x01}},
				{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 2}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 3}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5003, Timestamp: 4}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5004, Timestamp: 5}, Payload: []byte{0x02}},
				{Header: rtp.Header{SequenceNumber: 5005, Timestamp: 6}, Payload: []byte{0x03}},
			},
			headBytes: []byte{0x02},
			samples: []*Frame{
				{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 2},
				{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 3},
				{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 4},
				{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 5},
			},
			maxLate: 50,
		},
	}

	t.Run("Pop", func(t *testing.T) {
		assert := assert.New(t)

		for _, t := range testData {
			s := NewFrameBuilder(t.maxLate, &fakeDepacketizer{}, &fakePartitionHeadChecker{headBytes: t.headBytes})
			samples := []*Frame{}

			for _, p := range t.packets {
				s.Push(p)
			}
			for sample := s.Pop(); sample != nil; sample = s.Pop() {
				samples = append(samples, sample)
			}

			assert.Equal(t.samples, samples, t.message)
		}
	})
}

// FrameBuilder should respect maxLate if we popped successfully but then have a gap larger then maxLate
func TestFrameBuilderMaxLate(t *testing.T) {
	assert := assert.New(t)
	s := NewFrameBuilder(50, &fakeDepacketizer{}, &fakePartitionHeadChecker{headBytes: []byte{0x01}})

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0, Timestamp: 1}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1, Timestamp: 2}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2, Timestamp: 3}, Payload: []byte{0x03}})
	assert.Equal(&Frame{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 1}, s.Pop(), "Failed to build samples before gap")

	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5000, Timestamp: 500}, Payload: []byte{0x01}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5001, Timestamp: 501}, Payload: []byte{0x02}})
	s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 5002, Timestamp: 502}, Payload: []byte{0x03}})
	assert.Equal(&Frame{Packets: []interface{}{fakeDepacketizer{}}, Timestamp: 500}, s.Pop(), "Failed to build samples after large gap")
}

func TestSeqnumDistance(t *testing.T) {
	testData := []struct {
		x uint16
		y uint16
		d uint16
	}{
		{0x0001, 0x0003, 0x0002},
		{0x0003, 0x0001, 0x0002},
		{0xFFF3, 0xFFF1, 0x0002},
		{0xFFF1, 0xFFF3, 0x0002},
		{0xFFFF, 0x0001, 0x0002},
		{0x0001, 0xFFFF, 0x0002},
	}

	for _, data := range testData {
		if ret := seqnumDistance(data.x, data.y); ret != data.d {
			t.Errorf("seqnumDistance(%d, %d) returned %d which must be %d",
				data.x, data.y, ret, data.d)
		}
	}
}

func TestFrameBuilderCleanReference(t *testing.T) {
	for _, seqStart := range []uint16{
		0,
		0xFFF8, // check upper boundary
		0xFFFE, // check upper boundary
	} {
		seqStart := seqStart
		t.Run(fmt.Sprintf("From%d", seqStart), func(t *testing.T) {
			s := NewFrameBuilder(10, &fakeDepacketizer{}, &fakePartitionHeadChecker{})

			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 0 + seqStart, Timestamp: 0}, Payload: []byte{0x01}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 1 + seqStart, Timestamp: 0}, Payload: []byte{0x02}})
			s.Push(&rtp.Packet{Header: rtp.Header{SequenceNumber: 2 + seqStart, Timestamp: 0}, Payload: []byte{0x03}})
			pkt4 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 14 + seqStart, Timestamp: 120}, Payload: []byte{0x04}}
			s.Push(pkt4)
			pkt5 := &rtp.Packet{Header: rtp.Header{SequenceNumber: 12 + seqStart, Timestamp: 120}, Payload: []byte{0x05}}
			s.Push(pkt5)

			for i := 0; i < 3; i++ {
				if s.buffer[(i+int(seqStart))%0x10000] != nil {
					t.Errorf("Old packet (%d) is not unreferenced (maxLate: 10, pushed: 12)", i)
				}
			}
			if s.buffer[(14+int(seqStart))%0x10000] != pkt4 {
				t.Error("New packet must be referenced after jump")
			}
			if s.buffer[(12+int(seqStart))%0x10000] != pkt5 {
				t.Error("New packet must be referenced after jump")
			}
		})
	}
}
