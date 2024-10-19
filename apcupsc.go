// Package apcupsc implements an apcupsd client.
package apcupsc

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// timeFormat is the output format of apcupsd.
const timeFormat = "2006-01-02 15:04:05 -0700"

func parseTime(segs []string) (time.Time, error) {
	return time.Parse(timeFormat, strings.Join(segs, " "))
}

// TimeLocation is the default location for string formatted timestamps.
var TimeLocation = time.Local

// formatTime outputs a timestamp in the apcupsd format in the
// preferred time location. This detail allows apcupsd's to each be
// operating in their own time zone, but represents their values in a
// common time zone.
func formatTime(t time.Time) string {
	return t.In(TimeLocation).Format(timeFormat)
}

// Target holds the parsed summary of the APC output.
type Target struct {
	// Power consumption in Watts
	Power int
	// Charge in Watt Hours
	Charge int
	// Backup runtime in minutes
	Backup int
	// Charged, Offline, fully charged and on battery power
	Charged, Offline bool
	// Name of the UPS
	Name string
	// LineV is the current line voltage
	LineV float64
	// XFers is number of backup transitions
	XFers int
	// LastOnBattery time of last being on battery
	LastOnBattery time.Time
	// LastOutage is the string version of the outage
	LastOutage string
	// Lasted is how long the device was on battery
	Lasted time.Duration
	// Duration how long the device was on battery
	Duration string
}

// dialTimeout attempts to connect to an apcupsd endpoint.
func dialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	opt := net.Dialer{Timeout: timeout}
	conn, err := opt.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// DialDuration hold the timeout duration for connecting to an apcupsd service.
var DialDuration = time.Duration(4 * time.Second)

// ErrIncomplete indicates that the parsed target apcupsd returned
// truncated output.
var ErrIncomplete = errors.New("incomplete apcupsd read")

// ParseTarget attempts a connection to a target apdupsd address and
// returns sampled data as a *Target value, or nil when the target is
// unavailable with the corresponding error.
func ParseTarget(ep string) (*Target, error) {
	var nomPower, load float64
	var backup time.Duration
	t := &Target{}

	c, err := dialTimeout(ep, DialDuration)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// Tech spec sheets say:
	// 1500M = 187 WH Battery @ peak 900W - recharge 13W for 16 Hours
	// 1000M = 140 WH Battery @ peak 600W - recharge 12W for 12 Hours
	cmdStatus := []byte{0x00, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73}
	c.Write(cmdStatus)
	b := bufio.NewReader(c)
	fullRead := false
	for {
		line, _, err := b.ReadLine()
		if err != nil {
			continue
		}
		unpacked, err := decodeLine(line)
		if err != nil {
			continue
		}
		if len(unpacked) < 11 {
			continue
		}
		if strings.HasPrefix(unpacked, "END APC") {
			fullRead = true
			break
		}
		tokens := strings.Split(unpacked[11:], " ")
		if len(tokens) < 1 {
			continue
		}
		switch unpacked[:9] {
		case "NOMPOWER ":
			if len(tokens) != 2 && tokens[1] != "Watts" {
				continue
			}
			p, err := strconv.Atoi(tokens[0])
			if err != nil {
				continue
			}
			nomPower = float64(p)
		case "STATUS   ":
			t.Offline = tokens[0] != "ONLINE"
		case "TIMELEFT ":
			backup, _ = digestDuration(unpacked)
		case "NUMXFERS ":
			t.XFers, _ = strconv.Atoi(tokens[0])
		case "BCHARGE  ":
			t.Charged = tokens[0] == "100.0"
		case "LOADPCT  ":
			if len(tokens) != 2 || tokens[1] != "Percent" {
				continue
			}
			p, _ := strconv.ParseFloat(tokens[0], 64)
			load = p / 100
		case "LINEV    ":
			if len(tokens) != 2 || tokens[1] != "Volts" {
				continue
			}
			t.LineV, _ = strconv.ParseFloat(tokens[0], 64)
		case "END APC  ":
		case "UPSNAME  ":
			t.Name = tokens[0]
		case "XONBATT  ":
			t.LastOnBattery, err = parseTime(tokens[0:3])
			if err != nil {
				break
			}
			t.LastOutage = formatTime(t.LastOnBattery)
		case "XOFFBATT ":
			if len(tokens) < 3 {
				break
			}
			when, err := parseTime(tokens[0:3])
			if err != nil {
				break
			}
			d := when.Sub(t.LastOnBattery)
			if d <= 0 {
				break
			}
			t.Lasted = d
			t.Duration = d.String()
		default:
			continue
		}
	}

	if !fullRead {
		return nil, ErrIncomplete
	}

	t.Power = int(nomPower * load)
	mins := float64(backup / time.Minute)
	t.Charge = int(nomPower * load * mins / 60)
	t.Backup = int(mins)

	return t, nil
}

// ErrTooShort indicates that an apcupsd string return was too short
// to encode a string.
var ErrTooShort = errors.New("returned string too short")

// decodeLine decodes the apcupsd line encoding to return a string value.
func decodeLine(b []byte) (string, error) {
	if len(b) < 2 {
		return "", ErrTooShort
	}
	length := int(uint(b[0])*256 + uint(b[1]))
	if length != len(b)-1 {
		return "", fmt.Errorf("expected %d got %d:%q", length, len(b), b[2:])
	}
	return string(b[2:]), nil
}

// digestDuration consumes a string and converts it to a time.Duration.
func digestDuration(text string) (time.Duration, error) {
	if len(text) < 11 {
		return 0, fmt.Errorf("too short %d", len(text))
	}
	tokens := strings.Split(text[11:], " ")
	if len(tokens) != 2 {
		return 0, fmt.Errorf("want 2, got %d", len(tokens))
	}
	factor := 0.0
	switch strings.ToLower(tokens[1]) {
	case "minutes":
		factor = 60
	case "seconds":
		factor = 1
	default:
		return 0, fmt.Errorf("unrecognized time metric %q", tokens[1])
	}
	f, err := strconv.ParseFloat(tokens[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Second * time.Duration(factor*f), nil
}

// APCUPSDPort is the numerical port value for the apcupsd service.
var APCUPSDPort = 3551

// Scan scans a network for apcupsd services. The network string is
// provided in the format expected by net.ParseCIDR(). Scan returns a
// slice of full port addresses found. This function currently only
// support IPv4 networks.
func Scan(network string, timeout time.Duration) (ans []string) {
	_, nInfo, err := net.ParseCIDR(network)
	if err != nil || len(nInfo.Mask) != 4 {
		return
	}

	mask := binary.BigEndian.Uint32(nInfo.Mask)
	first := binary.BigEndian.Uint32(nInfo.IP)
	last := (first & mask) | ^mask

	var wg0 sync.WaitGroup
	var wg sync.WaitGroup
	ch := make(chan string)
	wg0.Add(1)
	go func() {
		defer wg0.Done()
		for r := range ch {
			ans = append(ans, r)
		}
	}()
	for n := first + 1; n <= last; n++ {
		var target string
		ip := make([]byte, 4)
		binary.BigEndian.PutUint32(ip, n)
		target = net.IP(ip).String()
		target = fmt.Sprint(target, ":", APCUPSDPort)
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := dialTimeout(target, timeout)
			if err != nil {
				return
			}
			defer c.Close()
			ch <- target
		}()
	}
	wg.Wait()
	close(ch)
	wg0.Wait()
	return
}
