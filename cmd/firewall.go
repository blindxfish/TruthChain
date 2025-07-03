package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// FirewallRule represents a Windows Firewall rule
type FirewallRule struct {
	Name        string
	Direction   string // "in" or "out"
	Protocol    string // "TCP" or "UDP"
	Port        int
	Program     string
	Description string
}

// ConfigureFirewall sets up Windows Firewall rules for TruthChain
func ConfigureFirewall(apiPort, meshPort int, programPath string) error {
	log.Println("🔧 Configuring Windows Firewall rules...")

	// Define the firewall rules we need
	rules := []FirewallRule{
		{
			Name:        "TruthChain API Server Inbound",
			Direction:   "in",
			Protocol:    "TCP",
			Port:        apiPort,
			Program:     programPath,
			Description: "Allow inbound connections to TruthChain API server",
		},
		{
			Name:        "TruthChain API Server Outbound",
			Direction:   "out",
			Protocol:    "TCP",
			Port:        apiPort,
			Program:     programPath,
			Description: "Allow outbound connections from TruthChain API server",
		},
		{
			Name:        "TruthChain Mesh Network Inbound",
			Direction:   "in",
			Protocol:    "TCP",
			Port:        meshPort,
			Program:     programPath,
			Description: "Allow inbound connections to TruthChain mesh network",
		},
		{
			Name:        "TruthChain Mesh Network Outbound",
			Direction:   "out",
			Protocol:    "TCP",
			Port:        meshPort,
			Program:     programPath,
			Description: "Allow outbound connections from TruthChain mesh network",
		},
	}

	// Add rules
	for _, rule := range rules {
		if err := addFirewallRule(rule); err != nil {
			log.Printf("Warning: Failed to add firewall rule '%s': %v", rule.Name, err)
			// Continue with other rules even if one fails
		} else {
			log.Printf("✅ Added firewall rule: %s", rule.Name)
		}
	}

	log.Println("🔧 Firewall configuration completed")
	return nil
}

// addFirewallRule adds a single Windows Firewall rule
func addFirewallRule(rule FirewallRule) error {
	// Check if rule already exists
	if ruleExists(rule.Name) {
		log.Printf("ℹ️  Firewall rule '%s' already exists, skipping", rule.Name)
		return nil
	}

	// Build the netsh command
	args := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + rule.Name,
		"dir=" + rule.Direction,
		"protocol=" + rule.Protocol,
		"localport=" + fmt.Sprintf("%d", rule.Port),
		"action=allow",
		"program=" + rule.Program,
		"description=" + rule.Description,
		"enable=yes",
	}

	// Execute the command
	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add firewall rule: %w, output: %s", err, string(output))
	}

	return nil
}

// ruleExists checks if a firewall rule already exists
func ruleExists(ruleName string) bool {
	args := []string{
		"advfirewall", "firewall", "show", "rule",
		"name=" + ruleName,
	}

	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If command fails, assume rule doesn't exist
		return false
	}

	// Check if the rule name appears in the output
	return strings.Contains(string(output), ruleName)
}

// RemoveFirewallRules removes TruthChain firewall rules
func RemoveFirewallRules() error {
	log.Println("🧹 Removing TruthChain firewall rules...")

	ruleNames := []string{
		"TruthChain API Server Inbound",
		"TruthChain API Server Outbound",
		"TruthChain Mesh Network Inbound",
		"TruthChain Mesh Network Outbound",
	}

	for _, ruleName := range ruleNames {
		if err := removeFirewallRule(ruleName); err != nil {
			log.Printf("Warning: Failed to remove firewall rule '%s': %v", ruleName, err)
		} else {
			log.Printf("✅ Removed firewall rule: %s", ruleName)
		}
	}

	log.Println("🧹 Firewall cleanup completed")
	return nil
}

// removeFirewallRule removes a single Windows Firewall rule
func removeFirewallRule(ruleName string) error {
	args := []string{
		"advfirewall", "firewall", "delete", "rule",
		"name=" + ruleName,
	}

	cmd := exec.Command("netsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove firewall rule: %w, output: %s", err, string(output))
	}

	return nil
}

// CheckFirewallStatus checks if firewall rules are properly configured
func CheckFirewallStatus(apiPort, meshPort int) map[string]interface{} {
	status := map[string]interface{}{
		"api_port":   apiPort,
		"mesh_port":  meshPort,
		"rules":      make(map[string]bool),
		"configured": false,
	}

	ruleNames := []string{
		"TruthChain API Server Inbound",
		"TruthChain API Server Outbound",
		"TruthChain Mesh Network Inbound",
		"TruthChain Mesh Network Outbound",
	}

	allConfigured := true
	for _, ruleName := range ruleNames {
		exists := ruleExists(ruleName)
		status["rules"].(map[string]bool)[ruleName] = exists
		if !exists {
			allConfigured = false
		}
	}

	status["configured"] = allConfigured
	return status
}

// ConfigureLinuxFirewall sets up Linux firewall rules for TruthChain
func ConfigureLinuxFirewall(apiPort, meshPort int) error {
	log.Println("🔧 Configuring Linux Firewall rules...")
	if isCommandAvailable("ufw") {
		return configureUFW(apiPort, meshPort)
	} else if isCommandAvailable("firewall-cmd") {
		return configureFirewalld(apiPort, meshPort)
	} else if isCommandAvailable("iptables") {
		return configureIptables(apiPort, meshPort)
	} else {
		return fmt.Errorf("No supported firewall tool found (ufw, firewalld, iptables)")
	}
}

func RemoveLinuxFirewallRules(apiPort, meshPort int) error {
	log.Println("🧹 Removing Linux Firewall rules...")
	if isCommandAvailable("ufw") {
		return removeUFW(apiPort, meshPort)
	} else if isCommandAvailable("firewall-cmd") {
		return removeFirewalld(apiPort, meshPort)
	} else if isCommandAvailable("iptables") {
		return removeIptables(apiPort, meshPort)
	} else {
		return fmt.Errorf("No supported firewall tool found (ufw, firewalld, iptables)")
	}
}

func CheckLinuxFirewallStatus(apiPort, meshPort int) map[string]interface{} {
	status := map[string]interface{}{
		"api_port":   apiPort,
		"mesh_port":  meshPort,
		"rules":      make(map[string]bool),
		"configured": false,
	}
	if isCommandAvailable("ufw") {
		return checkUFWStatus(apiPort, meshPort)
	} else if isCommandAvailable("firewall-cmd") {
		return checkFirewalldStatus(apiPort, meshPort)
	} else if isCommandAvailable("iptables") {
		return checkIptablesStatus(apiPort, meshPort)
	} else {
		status["note"] = "No supported firewall tool found (ufw, firewalld, iptables)"
		return status
	}
}

// Helper: check if a command exists
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// --- UFW ---
func configureUFW(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, dir := range []string{"allow in", "allow out"} {
			cmd := exec.Command("ufw", strings.Split(fmt.Sprintf("%s %d/tcp", dir, port), " ")...)
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Warning: ufw %s for port %d failed: %v, output: %s", dir, port, err, string(output))
			}
		}
	}
	return nil
}
func removeUFW(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, dir := range []string{"delete allow in", "delete allow out"} {
			cmd := exec.Command("ufw", strings.Split(fmt.Sprintf("%s %d/tcp", dir, port), " ")...)
			cmd.CombinedOutput() // ignore errors
		}
	}
	return nil
}
func checkUFWStatus(apiPort, meshPort int) map[string]interface{} {
	status := map[string]interface{}{
		"api_port":   apiPort,
		"mesh_port":  meshPort,
		"rules":      make(map[string]bool),
		"configured": false,
	}
	cmd := exec.Command("ufw", "status", "numbered")
	output, _ := cmd.CombinedOutput()
	out := string(output)
	allConfigured := true
	for _, port := range []int{apiPort, meshPort} {
		for _, dir := range []string{"ALLOW IN", "ALLOW OUT"} {
			key := fmt.Sprintf("%d/tcp %s", port, dir)
			found := strings.Contains(out, fmt.Sprintf("%d/tcp", port)) && strings.Contains(out, dir)
			status["rules"].(map[string]bool)[key] = found
			if !found {
				allConfigured = false
			}
		}
	}
	status["configured"] = allConfigured
	return status
}

// --- firewalld ---
func configureFirewalld(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, zone := range []string{"public"} {
			cmd := exec.Command("firewall-cmd", "--zone="+zone, "--add-port", fmt.Sprintf("%d/tcp", port), "--permanent")
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Warning: firewalld add-port %d failed: %v, output: %s", port, err, string(output))
			}
		}
	}
	// Reload firewalld
	cmd := exec.Command("firewall-cmd", "--reload")
	cmd.CombinedOutput()
	return nil
}
func removeFirewalld(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, zone := range []string{"public"} {
			cmd := exec.Command("firewall-cmd", "--zone="+zone, "--remove-port", fmt.Sprintf("%d/tcp", port), "--permanent")
			cmd.CombinedOutput()
		}
	}
	cmd := exec.Command("firewall-cmd", "--reload")
	cmd.CombinedOutput()
	return nil
}
func checkFirewalldStatus(apiPort, meshPort int) map[string]interface{} {
	status := map[string]interface{}{
		"api_port":   apiPort,
		"mesh_port":  meshPort,
		"rules":      make(map[string]bool),
		"configured": false,
	}
	cmd := exec.Command("firewall-cmd", "--list-ports")
	output, _ := cmd.CombinedOutput()
	out := string(output)
	allConfigured := true
	for _, port := range []int{apiPort, meshPort} {
		key := fmt.Sprintf("%d/tcp", port)
		found := strings.Contains(out, key)
		status["rules"].(map[string]bool)[key] = found
		if !found {
			allConfigured = false
		}
	}
	status["configured"] = allConfigured
	return status
}

// --- iptables ---
func configureIptables(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, chain := range []string{"INPUT", "OUTPUT"} {
			cmd := exec.Command("iptables", "-A", chain, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Warning: iptables %s for port %d failed: %v, output: %s", chain, port, err, string(output))
			}
		}
	}
	return nil
}
func removeIptables(apiPort, meshPort int) error {
	for _, port := range []int{apiPort, meshPort} {
		for _, chain := range []string{"INPUT", "OUTPUT"} {
			cmd := exec.Command("iptables", "-D", chain, "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
			cmd.CombinedOutput()
		}
	}
	return nil
}
func checkIptablesStatus(apiPort, meshPort int) map[string]interface{} {
	status := map[string]interface{}{
		"api_port":   apiPort,
		"mesh_port":  meshPort,
		"rules":      make(map[string]bool),
		"configured": false,
	}
	cmd := exec.Command("iptables", "-L")
	output, _ := cmd.CombinedOutput()
	out := string(output)
	allConfigured := true
	for _, port := range []int{apiPort, meshPort} {
		key := fmt.Sprintf("%d/tcp", port)
		found := strings.Contains(out, fmt.Sprintf("dpt:%d", port))
		status["rules"].(map[string]bool)[key] = found
		if !found {
			allConfigured = false
		}
	}
	status["configured"] = allConfigured
	return status
}
