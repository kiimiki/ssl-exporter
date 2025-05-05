package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"ssl-exporter/ssl"
)

type DomainList struct {
	Domains []string `json:"domains"`
}

const metricsFile = "metrics"

func LoadDomains(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list DomainList
	err = json.Unmarshal(data, &list)
	return list.Domains, err
}

func Generate(domains []string) {
	log.Println("Generating metrics...")

	content := "# HELP ssl_cert_days_left Days left to expiration\n" +
		"# TYPE ssl_cert_days_left gauge\n" +
		"# HELP ssl_cert_start_timestamp Certificate start date (Unix timestamp)\n" +
		"# TYPE ssl_cert_start_timestamp gauge\n" +
		"# HELP ssl_cert_end_timestamp Certificate expiry date (Unix timestamp)\n" +
		"# TYPE ssl_cert_end_timestamp gauge\n" +
		"# HELP ssl_cert_domain_colored Used for domain coloring in Grafana\n" +
		"# TYPE ssl_cert_domain_colored gauge\n"

	for _, domain := range domains {
		cleanDomain := domain
		ftpLabel := "false"
		if strings.HasPrefix(domain, "ftp://") {
			cleanDomain = strings.TrimPrefix(domain, "ftp://")
			ftpLabel = "true"
		}

		var start, end time.Time
		var err error

		if ftpLabel == "true" {
			log.Printf("Checking FTP TLS for: %s", cleanDomain)
			start, end, err = ssl.GetFTPCertificateTimestamps(cleanDomain)
		} else {
			log.Printf("Checking HTTPS TLS for: %s", cleanDomain)
			start, end, err = ssl.GetCertificateTimestamps(cleanDomain)
		}

		if err != nil {
			log.Printf("[ERROR] %s: %v", cleanDomain, err)
			content += fmt.Sprintf("ssl_cert_days_left{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_domain_colored{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_start_timestamp{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_end_timestamp{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			continue
		}

		daysLeft := int(end.Sub(time.Now()).Hours() / 24)
		log.Printf("[OK] %s: Start: %s, End: %s, Days left: %d", cleanDomain, start, end, daysLeft)

		content += fmt.Sprintf("ssl_cert_days_left{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, daysLeft)
		content += fmt.Sprintf("ssl_cert_domain_colored{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, daysLeft)
		content += fmt.Sprintf("ssl_cert_start_timestamp{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, start.Unix())
		content += fmt.Sprintf("ssl_cert_end_timestamp{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, end.Unix())
	}

	_ = os.WriteFile(metricsFile, []byte(content), 0644)
}
