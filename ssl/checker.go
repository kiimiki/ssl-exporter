package ssl

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	attempts  = make(map[string]int)
	durations = make(map[string]float64)
	failures  = make(map[string]float64)
	errors    = make(map[string]string)
	mu        sync.Mutex
)

func GetAttemptMetrics() map[string]int             { return attempts }
func GetDurationMetrics() map[string]float64        { return durations }
func GetFailureDurationMetrics() map[string]float64 { return failures }
func GetErrorMessages() map[string]string           { return errors }

func recordMetrics(key string, success bool, duration float64, err error) {
	mu.Lock()
	defer mu.Unlock()
	attempts[key]++
	if success {
		durations[key] = duration
	} else {
		failures[key] = duration
		if err != nil {
			errors[key] = err.Error()
		}
	}
}

func GetCertificateTimestamps(domain string) (time.Time, time.Time, error) {
	return getTLSCertificate(domain, "443", "https")
}

func GetFTPCertificateTimestamps(domain string) (time.Time, time.Time, error) {
	startTime := time.Now()
	key := domain + "_ftp"
	serverName := strings.TrimPrefix(domain, "ftp://")

	rawConn, err := net.DialTimeout("tcp", serverName+":21", 10*time.Second)
	if err != nil {
		recordMetrics(key, false, time.Since(startTime).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}
	defer rawConn.Close()

	buff := make([]byte, 1024)
	rawConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	rawConn.Read(buff)

	_, err = rawConn.Write([]byte("AUTH TLS\r\n"))
	if err != nil {
		recordMetrics(key, false, time.Since(startTime).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}

	rawConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := rawConn.Read(buff)
	if err != nil || !strings.HasPrefix(string(buff[:n]), "234") {
		recordMetrics(key, false, time.Since(startTime).Seconds(), fmt.Errorf("AUTH TLS failed"))
		return time.Time{}, time.Time{}, fmt.Errorf("AUTH TLS failed")
	}

	tlsConn := tls.Client(rawConn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
	})
	if err := tlsConn.Handshake(); err != nil {
		recordMetrics(key, false, time.Since(startTime).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}
	defer tlsConn.Close()

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		recordMetrics(key, false, time.Since(startTime).Seconds(), fmt.Errorf("no certificate found"))
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate found")
	}

	recordMetrics(key, true, time.Since(startTime).Seconds(), nil)
	return certs[0].NotBefore, certs[0].NotAfter, nil
}

func getTLSCertificate(domain, port, proto string) (time.Time, time.Time, error) {
	startTime := time.Now()
	key := domain + "_" + proto

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":"+port, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err != nil {
		recordMetrics(key, false, time.Since(startTime).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		recordMetrics(key, false, time.Since(startTime).Seconds(), fmt.Errorf("no certificate found"))
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate found")
	}

	recordMetrics(key, true, time.Since(startTime).Seconds(), nil)
	return certs[0].NotBefore, certs[0].NotAfter, nil
}
