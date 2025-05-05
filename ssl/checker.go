package ssl

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

func GetCertificateTimestamps(domain string) (time.Time, time.Time, error) {
	return getTLSCertificate(domain, "443")
}

func GetFTPCertificateTimestamps(domain string) (time.Time, time.Time, error) {
	conn, err := net.DialTimeout("tcp", domain+":21", 5*time.Second)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer conn.Close()

	buff := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.Read(buff)

	conn.Write([]byte("AUTH TLS\r\n"))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buff)
	if err != nil || !strings.HasPrefix(string(buff[:n]), "234") {
		return time.Time{}, time.Time{}, fmt.Errorf("AUTH TLS failed")
	}

	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err := tlsConn.Handshake(); err != nil {
		return time.Time{}, time.Time{}, err
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate found")
	}
	return certs[0].NotBefore, certs[0].NotAfter, nil
}

func getTLSCertificate(domain, port string) (time.Time, time.Time, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":"+port, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         domain,
	})
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("no certificate found")
	}

	return certs[0].NotBefore, certs[0].NotAfter, nil
}
