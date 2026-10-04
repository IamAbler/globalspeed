package ping

import "testing"

func TestParseAPKStatistics(t *testing.T) {
	output := `64 bytes from 1.2.3.4: icmp_seq=1 ttl=54 time=10.0 ms
64 bytes from 1.2.3.4: icmp_seq=3 ttl=54 time=14.0 ms
64 bytes from 1.2.3.4: icmp_seq=4 ttl=54 time=12.0 ms
--- ping statistics ---
5 packets transmitted, 3 received, 40% packet loss
rtt min/avg/max/mdev = 10.000/12.000/14.000/1.000 ms`
	r := Parse(output, 5)
	if r.AverageMS != 12 || r.JitterMS != 3 || r.PacketLossPct != 40 || r.Received != 3 || r.Status != 0 {
		t.Fatalf("%+v", r)
	}
}
func TestTCPAPKIntegerStatistics(t *testing.T) {
	r := TCPStatistics([]int64{10, 14, 12}, 5)
	if r.AverageMS != 12 || r.JitterMS != 2 || r.Status != 0 || r.PacketLossPct != 40 || r.APKPackageLost == nil || *r.APKPackageLost != 0 {
		t.Fatalf("%+v", r)
	}
	r = TCPStatistics([]int64{0, 0}, 2)
	if r.AverageMS != 0 || r.Status != 999 {
		t.Fatalf("sub-ms connections must not be promoted: %+v", r)
	}
}
func TestWindowsReplies(t *testing.T) {
	r := Parse("来自 1.2.3.4 的回复: 字节=64 时间=12ms TTL=54\n来自 1.2.3.4 的回复: 字节=64 时间=14ms TTL=54", 5)
	if r.AverageMS != 13 || r.JitterMS != 2 || r.Received != 2 {
		t.Fatalf("%+v", r)
	}
	r = Parse("Reply from 1.2.3.4: bytes=64 time<1ms TTL=54", 5)
	if r.AverageMS != 0 || r.Status != 999 {
		t.Fatalf("upper bound must not be a fabricated RTT: %+v", r)
	}
}
