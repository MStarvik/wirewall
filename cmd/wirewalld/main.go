package main

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gopkg.in/ini.v1"
)

type Config struct {
	Interface string
	Zone      string
}

type Client struct {
	Name         string
	IP           net.IP
	PublicKey    wgtypes.Key
	PresharedKey *wgtypes.Key
}

func (c Client) FQDN(zone string) string {
	return c.Name + "." + zone
}

func (c Client) PTR(zone string) string {
	return c.IP.String() + ".in-addr.arpa"
}

func (c Client) PeerConfig() wgtypes.PeerConfig {
	return wgtypes.PeerConfig{
		PublicKey:         c.PublicKey,
		PresharedKey:      c.PresharedKey,
		ReplaceAllowedIPs: true,
		AllowedIPs:        []net.IPNet{{IP: c.IP, Mask: net.CIDRMask(32, 32)}},
	}
}

func ReadConfig(file string) (Config, error) {
	cfg, err := ini.Load(file)
	if err != nil {
		return Config{}, err
	}

	section := cfg.Section("")

	if !section.HasKey("interface") {
		return Config{}, errors.New("read " + file + ": missing interface")
	}

	if !section.HasKey("zone") {
		return Config{}, errors.New("read " + file + ": missing zone")
	}

	return Config{
		Interface: section.Key("interface").String(),
		Zone:      section.Key("zone").String(),
	}, nil

}

func readClient(file string) (*Client, error) {
	_, name := path.Split(file)
	name = strings.TrimSuffix(name, ".conf")

	cfg, err := ini.Load(file)
	if err != nil {
		return nil, err
	}

	section := cfg.Section("")

	if !section.HasKey("ip") {
		return nil, errors.New("read " + file + ": missing ip")
	}

	ip := net.ParseIP(section.Key("ip").String())
	if ip == nil {
		return nil, errors.New("read " + file + ": invalid ip")
	}

	if !section.HasKey("public_key") {
		return nil, errors.New("read " + file + ": missing public_key")
	}

	publicKey, err := wgtypes.ParseKey(section.Key("public_key").String())
	if err != nil {
		return nil, err
	}

	var presharedKey *wgtypes.Key
	if section.HasKey("preshared_key") {
		key, err := wgtypes.ParseKey(section.Key("preshared_key").String())
		if err != nil {
			return nil, err
		}
		presharedKey = &key
	}

	return &Client{name, ip, publicKey, presharedKey}, nil
}

func readClients(dir string) ([]*Client, error) {
	clients := []*Client{}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".conf") {
			continue
		}

		client, err := readClient(path.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}

		clients = append(clients, client)
	}

	return clients, nil
}

func configureWG(iface string, clients []*Client) error {
	wg, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wg.Close()

	config := wgtypes.Config{}
	for _, client := range clients {
		config.Peers = append(config.Peers, client.PeerConfig())
	}

	if err := wg.ConfigureDevice(iface, config); err != nil {
		return err
	}

	return nil
}

func updateDNS(zone string, clients []*Client) error {
	cmd := exec.Command("nsupdate", "-l")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	if err := cmd.Start(); err != nil {
		return err
	}

	io.WriteString(stdin, "zone "+zone+"\n")
	for _, client := range clients {
		io.WriteString(stdin, "update add "+client.FQDN(zone)+" 3600 A "+client.IP.String()+"\n")
	}
	io.WriteString(stdin, "send\n")

	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func main() {
	config, err := ReadConfig("/etc/wirewall/wirewall.conf")
	if err != nil {
		panic(err)
	}

	clients, err := readClients("/etc/wirewall/clients")
	if err != nil {
		panic(err)
	}

	if err := configureWG(config.Interface, clients); err != nil {
		panic(err)
	}

	if err := updateDNS(config.Zone, clients); err != nil {
		panic(err)
	}

	// conn, err := nftables.New()
	// if err != nil {
	// 	panic(err)
	// }

	// chains, err := conn.ListChains()
	// if err != nil {
	// 	panic(err)
	// }

	// for _, chain := range chains {
	// 	println(chain.Name)
	// }
}
