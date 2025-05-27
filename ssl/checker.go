package ssl

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	attempts = make(map[string]int)
	success  = make(map[string]float64)
	failures = make(map[string]float64)
	errors   = make(map[string]string)
)

func GetAttempts() map[string]int             { return attempts }
func GetSuccessDurations() map[string]float64 { return success }
func GetFailureDurations() map[string]float64 { return failures }
func GetErrors() map[string]string            { return errors }

func record(domain string, ok bool, dur float64, err error) {
	mu.Lock()
	defer mu.Unlock()
	attempts[domain]++
	if ok {
		success[domain] = dur
	} else {
		failures[domain] = dur
		if err != nil {
			errors[domain] = err.Error()
		}
	}
}

func GetCertificate(domain, proto string) (time.Time, time.Time, error) {
	if proto == "ftp" {
		return getFTPAutoTLSCert(domain)
	}
	return getTLScert(domain, "443")
}

func getTLScert(domain, port string) (time.Time, time.Time, error) {
	start := time.Now()
	conn, err := tls.Dial("tcp", domain+":"+port, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err != nil {
		record(domain+"_https", false, time.Since(start).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		record(domain+"_https", false, time.Since(start).Seconds(), fmt.Errorf("no certificate"))
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate")
	}

	record(domain+"_https", true, time.Since(start).Seconds(), nil)
	return certs[0].NotBefore, certs[0].NotAfter, nil
}

func getFTPCertAutoDetect(domain string) (time.Time, time.Time, error) {
	start := time.Now()

	// Шаг 1 — обычное подключение
	conn, err := net.DialTimeout("tcp", domain+":21", 10*time.Second)
	if err != nil {
		record(domain+"_ftp", false, time.Since(start).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 4096)
	_, err = conn.Read(buf) // читаем баннер
	if err != nil {
		conn.Close()
		goto tryAutoTLS
	}

	// Шаг 2 — пробуем AUTH TLS
	_, err = conn.Write([]byte("AUTH TLS\r\n"))
	if err != nil {
		conn.Close()
		goto tryAutoTLS
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		goto tryAutoTLS
	}

	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "234") {
		conn.Close()
		goto tryAutoTLS
	}

	// Шаг 3 — STARTTLS
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		goto tryAutoTLS
	}
	defer tlsConn.Close()

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		record(domain+"_ftp", false, time.Since(start).Seconds(), fmt.Errorf("no certificate"))
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate")
	}

	record(domain+"_ftp", true, time.Since(start).Seconds(), nil)
	return certs[0].NotBefore, certs[0].NotAfter, nil

tryAutoTLS:
	// fallback на auto-TLS
	tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", domain+":21", &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err != nil {
		record(domain+"_ftp", false, time.Since(start).Seconds(), err)
		return time.Time{}, time.Time{}, err
	}
	defer tlsConn.Close()

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		record(domain+"_ftp", false, time.Since(start).Seconds(), fmt.Errorf("no certificate"))
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate")
	}

	record(domain+"_ftp", true, time.Since(start).Seconds(), nil)
	return certs[0].NotBefore, certs[0].NotAfter, nil
}
