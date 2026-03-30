package process

import (
	"bufio"
	"os"
)

func TailLog(logPath string, lines int) ([]string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}

	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return all, scanner.Err()
}
