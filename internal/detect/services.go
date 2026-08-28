package detect

// serviceNames maps well-known ports to the service that is registered or
// almost universally used on them. The set is curated and conservative: only
// stable IANA-registered services and widely deployed infrastructure are
// included, so the mapping reads as a fact ("port 5432 is the well-known
// PostgreSQL port"), never as a claim about what a process is doing.
var serviceNames = map[uint16]string{
	20: "FTP-data", 21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP",
	53: "DNS", 67: "DHCP", 68: "BOOTP", 69: "TFTP", 80: "HTTP",
	88: "Kerberos", 110: "POP3", 123: "NTP", 135: "MS-RPC",
	137: "NetBIOS-NS", 138: "NetBIOS-DGM", 139: "NetBIOS-SSN", 143: "IMAP",
	161: "SNMP", 162: "SNMP-trap", 389: "LDAP", 443: "HTTPS", 445: "SMB",
	465: "SMTPS", 500: "IPsec/IKE", 514: "syslog", 515: "LPD (printing)",
	548: "AFP", 587: "SMTP submission", 631: "IPP (printing)",
	636: "LDAPS", 873: "rsync", 993: "IMAPS", 995: "POP3S", 1080: "SOCKS",
	1433: "MSSQL", 1434: "MSSQL browser", 1521: "Oracle DB",
	1723: "PPTP", 2049: "NFS", 2181: "ZooKeeper", 2375: "Docker API",
	2376: "Docker API (TLS)", 3128: "HTTP proxy", 3268: "LDAP GC", 3306: "MySQL",
	3389: "RDP", 4369: "Erlang Port Mapper", 5000: "AirPlay/Dev",
	5001: "AirPlay", 5050: "Mesos", 5228: "Google Push", 5353: "mDNS (DNS-SD)",
	5432: "PostgreSQL", 5900: "Screen Sharing (VNC)", 5984: "CouchDB",
	6000: "X11", 6379: "Redis", 6667: "IRC", 8000: "HTTP-alt",
	8080: "HTTP-alt", 8081: "HTTP-alt", 8086: "InfluxDB", 8087: "HTTP-alt",
	8443: "HTTPS-alt", 9092: "Kafka",
	9200: "Elasticsearch", 9300: "Elasticsearch", 9418: "Git",
	11211: "Memcached", 27017: "MongoDB", 28017: "MongoDB (http)",
	3283: "Apple Remote Desktop",
}

// LookupService returns the well-known service name for a port, or "" when the
// port has no entry in the registry.
func LookupService(port uint16) string {
	return serviceNames[port]
}
