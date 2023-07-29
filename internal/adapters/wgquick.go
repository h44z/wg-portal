package adapters

import (
	"bytes"
	"fmt"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	"os/exec"
	"strings"
)

// WgQuickRepo implements higher level wg-quick like interactions like setting DNS, routing tables or interface hooks.
type WgQuickRepo struct {
	shellCmd              string
	resolvConfIfacePrefix string
}

func NewWgQuickRepo() *WgQuickRepo {
	return &WgQuickRepo{
		shellCmd:              "bash",
		resolvConfIfacePrefix: "tun.",
	}
}

func (r *WgQuickRepo) ExecuteInterfaceHook(id domain.InterfaceIdentifier, hookCmd string) error {
	if hookCmd == "" {
		return nil
	}

	err := r.exec(hookCmd, id)
	if err != nil {
		return fmt.Errorf("failed to exec hook: %w", err)
	}

	return nil
}

func (r *WgQuickRepo) SetDNS(id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error {
	if dnsStr == "" && dnsSearchStr == "" {
		return nil
	}

	dnsServers := internal.SliceString(dnsStr)
	dnsSearchDomains := internal.SliceString(dnsSearchStr)

	dnsCommand := "resolvconf -a %resPref%i -m 0 -x"
	dnsCommandInput := make([]string, 0, len(dnsServers)+len(dnsSearchDomains))

	for _, dnsServer := range dnsServers {
		dnsCommandInput = append(dnsCommandInput, fmt.Sprintf("nameserver %s", dnsServer))
	}
	for _, searchDomain := range dnsSearchDomains {
		dnsCommandInput = append(dnsCommandInput, fmt.Sprintf("search %s", searchDomain))
	}

	err := r.exec(dnsCommand, id, dnsCommandInput...)
	if err != nil {
		return fmt.Errorf("failed to set dns settings: %w", err)
	}

	return nil
}

func (r *WgQuickRepo) UnsetDNS(id domain.InterfaceIdentifier) error {
	dnsCommand := "resolvconf -d %resPref%i -f"

	err := r.exec(dnsCommand, id)
	if err != nil {
		return fmt.Errorf("failed to unset dns settings: %w", err)
	}

	return nil
}

func (r *WgQuickRepo) replaceCommandPlaceHolders(command string, interfaceId domain.InterfaceIdentifier) string {
	command = strings.ReplaceAll(command, "%resPref", r.resolvConfIfacePrefix)
	return strings.ReplaceAll(command, "%i", string(interfaceId))
}

func (r *WgQuickRepo) exec(command string, interfaceId domain.InterfaceIdentifier, stdin ...string) error {
	commandWithInterfaceName := r.replaceCommandPlaceHolders(command, interfaceId)
	cmd := exec.Command(r.shellCmd, "-ce", commandWithInterfaceName)
	if len(stdin) > 0 {
		b := &bytes.Buffer{}
		for _, ln := range stdin {
			if _, err := fmt.Fprint(b, ln); err != nil {
				return err
			}
		}
		cmd.Stdin = b
	}
	out, err := cmd.CombinedOutput() // execute and wait for output
	if err != nil {
		return fmt.Errorf("failed to exexute shell command %s: %w", commandWithInterfaceName, err)
	}
	logrus.Tracef("executed shell command %s, with output: %s", commandWithInterfaceName, string(out))
	return nil
}
