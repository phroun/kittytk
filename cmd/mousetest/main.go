package main

import (
	"fmt"
	"time"
)

func main() {
	// Enable mouse modes (same as direct-key-handler test app)
	fmt.Print("\033[?1000h") // Basic mouse tracking
	fmt.Print("\033[?1002h") // Button event + motion tracking
	fmt.Print("\033[?1006h") // SGR mouse mode

	fmt.Println("Mouse mode enabled. Move your mouse or click...")
	fmt.Println("Waiting 3 seconds then exiting WITHOUT disabling mouse mode.")
	fmt.Println("After exit, mouse events should appear as escape sequences on prompt.")

	time.Sleep(3 * time.Second)

	fmt.Println("Exiting (mouse mode still enabled)...")
	// Intentionally NOT disabling mouse mode so user can see events at prompt
}
