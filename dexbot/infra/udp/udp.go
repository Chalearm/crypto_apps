/******************************************************************************
 * File Name       : udp.go
 * File Path       : infra/udp/udp.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   Provides utilities for sending and receiving UDP messages between daemons. Import and use `udp.SendMessage` to send data and `udp.Listen` to receive data. Updated Part: - Initial creation of the UDP c
 *
 * Responsibilities:
 *   - Implement core functionality for infra package.
 *
 * Usage :
 *   Directory : infra/udp/
 *
 *   Build :
 *     go build ./infra/udp
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./infra/udp
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Functions] All exported functions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package udp

import (
    "fmt"
    "net"
    "time"
    "dexbot/infra"
)

const (
    readBufferSize = 1024
)

// SendMessage sends a byte slice message to a specified UDP address.
// Purpose: Facilitates inter-daemon communication via UDP.
// Input:
//   addr string: The target UDP address (e.g., "127.0.0.1:8081").
//   message []byte: The byte slice message to send.
// Output:
//   error: An error if the message cannot be sent.
// Lines: ~20
func SendMessage(addr string, message []byte) error {
    udpAddr, err := net.ResolveUDPAddr("udp", addr)
    if err != nil {
        return fmt.Errorf("failed to resolve UDP address %s: %w", addr, err)
    }

    conn, err := net.DialUDP("udp", nil, udpAddr)
    if err != nil {
        return fmt.Errorf("failed to dial UDP %s: %w", addr, err)
    }
    defer conn.Close()

    _, err = conn.Write(message)
    if err != nil {
        return fmt.Errorf("failed to send UDP message to %s: %w", addr, err)
    }

    return nil
}

// Listen starts a UDP listener on the specified port and calls a handler function for each received message.
// Purpose: Allows daemons to receive messages from other daemons.
// Input:
//   port int: The UDP port to listen on.
//   handler func([]byte, *net.UDPAddr): A function to process incoming messages.
//   stopChan <-chan struct{}: A channel to signal the listener to stop.
// Output:
//   error: An error if the listener cannot be started.
// Lines: ~40
func Listen(port int, handler func([]byte, *net.UDPAddr), stopChan <-chan struct{}) error {
    addr := fmt.Sprintf(":%d", port)
    udpAddr, err := net.ResolveUDPAddr("udp", addr)
    if err != nil {
        return fmt.Errorf("failed to resolve UDP address %s: %w", addr, err)
    }

    conn, err := net.ListenUDP("udp", udpAddr)
    if err != nil {
        return fmt.Errorf("failed to listen on UDP %s: %w", addr, err)
    }
    defer conn.Close()

    infra.Info(fmt.Sprintf("UDP listener started on port %d", port))

    buffer := make([]byte, readBufferSize)

    for {
        select {
        case <-stopChan:
            infra.Info(fmt.Sprintf("UDP listener on port %d stopping", port))
            return nil
        default:
            // Set a read deadline to allow checking stopChan
            conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
            n, remoteAddr, err := conn.ReadFromUDP(buffer)
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue // Timeout, check stopChan again
                } else {
                    infra.Error(fmt.Sprintf("UDP read error: %v", err))
                    return fmt.Errorf("UDP read error: %w", err)
                }
            }
            handler(buffer[:n], remoteAddr)
        }
    }
}
