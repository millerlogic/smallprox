// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// LoadHosts loads one host per line or in /etc/hosts format.
func LoadHosts(f io.Reader) ([]string, error) {
	var hosts []string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		ent := strings.TrimSpace(scan.Text())
		ihash := strings.IndexByte(ent, '#')
		if ihash != -1 {
			ent = strings.TrimSpace(ent[:ihash])
		}
		if len(ent) == 0 {
			continue
		}
		parts := strings.Fields(ent)
		switch len(parts) {
		case 0:
		case 1:
			hosts = append(hosts, parts[0])
		default: // Treat as /etc/hosts format:
			for _, host := range parts[1:] {
				hosts = append(hosts, host)
			}
		}
	}
	err := scan.Err()
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

// LoadHostsFile loads hosts from the file, see LoadHosts
func LoadHostsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadHosts(f)
}

// ContainsHost returns true if host is found in hosts.
func ContainsHost(hosts []string, host string) bool {
	return HasAnyFold(hosts, host)
}
