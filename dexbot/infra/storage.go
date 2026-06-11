/*
Filename: infra/storage.go
*/

package infra

import (
    "os"
)

func SaveLocal(file string, data string) error {
    return os.WriteFile(file, []byte(data), 0644)
}
