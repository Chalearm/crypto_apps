/*
Filename: infra/disk_test.go
*/
package infra

import "testing"

func TestDiskValue(t *testing.T) {
    val := GetFreeDiskPercent()
    if val <= 0 {
        t.Error("invalid disk percent")
    }
}
