package h264

import (
	"encoding/binary"
	"testing"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

// fragmentedIDR returns one big AVCC IDR NAL and its FU-A RTP packets.
func fragmentedIDR(t *testing.T) (avcc []byte, packets []*rtp.Packet) {
	t.Helper()

	nalu := make([]byte, 3000)
	nalu[0] = 0x65 // IDR, nal_ref_idc=3
	for i := 1; i < len(nalu); i++ {
		nalu[i] = byte(i)
	}
	avcc = make([]byte, 4+len(nalu))
	binary.BigEndian.PutUint32(avcc, uint32(len(nalu)))
	copy(avcc[4:], nalu)

	payloader := &Payloader{IsAVC: true}
	payloads := payloader.Payload(200, avcc)
	require.Greater(t, len(payloads), 3, "expected the NAL to be fragmented")

	for i, p := range payloads {
		packets = append(packets, &rtp.Packet{
			Header:  rtp.Header{Version: 2, Marker: i == len(payloads)-1, SequenceNumber: uint16(i)},
			Payload: p,
		})
	}
	return
}

func depay(t *testing.T) (core.HandlerFunc, *[][]byte) {
	t.Helper()
	var got [][]byte
	handler := RTPDepay(&core.Codec{Name: core.CodecH264}, func(packet *rtp.Packet) {
		got = append(got, packet.Payload)
	})
	return handler, &got
}

// A consumer that attaches while an FU-A fragmented NAL is in flight has no
// slice header for it; the tail must be dropped, not passed on as a bogus NAL.
func TestRTPDepayDropsFUATailWithoutStart(t *testing.T) {
	avcc, packets := fragmentedIDR(t)
	handler, got := depay(t)

	// join mid-NAL: skip the fragment carrying the start bit
	for _, p := range packets[2:] {
		handler(p)
	}
	require.Empty(t, *got, "tail fragments without a start bit produced a NAL")

	// the next complete NAL must still come through intact
	for _, p := range packets {
		handler(p)
	}
	require.Len(t, *got, 1)
	require.Equal(t, avcc, (*got)[0])
}

// A start fragment arriving while a previous fragment set never finished
// (lost end bit) must discard the stale partial data.
func TestRTPDepayRestartsOnNewFUAStart(t *testing.T) {
	avcc, packets := fragmentedIDR(t)
	handler, got := depay(t)

	// first NAL loses its tail
	for _, p := range packets[:len(packets)-2] {
		handler(p)
	}
	// second NAL arrives complete
	for _, p := range packets {
		handler(p)
	}
	require.Len(t, *got, 1)
	require.Equal(t, avcc, (*got)[0])
}
