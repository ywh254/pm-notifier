package main

import (
	"os"

	"github.com/stefantds/csvdecoder"
)

type Member struct {
	Name  string
	Phone string
}

func readMembers(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder, err := csvdecoder.New(file)
	if err != nil {
		return nil, err
	}

	members := make(map[string]string)
	for decoder.Next() {
		var m Member

		if err := decoder.Scan(&m.Name, &m.Phone); err != nil {
			return nil, err
		}

		members[m.Name] = m.Phone
	}

	if err = decoder.Err(); err != nil {
		return nil, err
	}

	return members, nil
}
