package main

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gopkg.in/ini.v1"
)

const configFile = "/etc/wirewall/wirewall.conf"
const clientsDir = "/etc/wirewall/clients"

const dbusName = "no.mstarvik.wirewall"
const dbusPath = "/no/mstarvik/wirewall"
const dbusIntro = `
<node>
	<interface name="no.mstarvik.wirewall">
		<method name="Configure">
			<arg type="s" name="error" direction="out"/>
		</method>
		<method name="Reload">
			<arg type="s" name="error" direction="out"/>
		</method>
	</interface>
</node>
`

type Config struct {
	Interface string
	Zone      *string
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

func ReadConfig(file string) (*Config, error) {
	config := new(Config)

	cfg, err := ini.Load(file)
	if err != nil {
		return nil, err
	}

	section := cfg.Section("")

	if !section.HasKey("interface") {
		return nil, errors.New("read " + file + ": missing interface")
	}
	config.Interface = section.Key("interface").String()

	if section.HasKey("zone") {
		zone := section.Key("zone").String()
		config.Zone = &zone
	}

	return config, nil

}

func readClient(file string) (*Client, error) {
	client := new(Client)

	_, name := path.Split(file)
	client.Name = strings.TrimSuffix(name, ".conf")

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
	client.IP = ip

	if !section.HasKey("public_key") {
		return nil, errors.New("read " + file + ": missing public_key")
	}

	publicKey, err := wgtypes.ParseKey(section.Key("public_key").String())
	if err != nil {
		return nil, err
	}
	client.PublicKey = publicKey

	if section.HasKey("preshared_key") {
		key, err := wgtypes.ParseKey(section.Key("preshared_key").String())
		if err != nil {
			return nil, err
		}
		client.PresharedKey = &key
	}

	return client, nil
}

func readClients(dir string) ([]Client, error) {
	clients := []Client{}

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

		clients = append(clients, *client)
	}

	return clients, nil
}

func configureWG(iface string, clients []Client) error {
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

func updateDNS(zone string, clients []Client) error {
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

type WireWall struct {
	Config  Config
	Clients []Client
}

func (state *WireWall) Configure() *dbus.Error {
	clients, err := readClients(clientsDir)
	if err != nil {
		return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
	}
	state.Clients = clients

	if err := configureWG(state.Config.Interface, clients); err != nil {
		return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
	}

	if state.Config.Zone != nil {
		if err := updateDNS(*state.Config.Zone, clients); err != nil {
			return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
		}
	}

	return nil
}

func (state *WireWall) Reload() *dbus.Error {
	config, err := ReadConfig(configFile)
	if err != nil {
		return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
	}
	state.Config = *config

	clients, err := readClients(clientsDir)
	if err != nil {
		return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
	}
	state.Clients = clients

	if err := configureWG(state.Config.Interface, clients); err != nil {
		return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
	}

	if state.Config.Zone != nil {
		if err := updateDNS(*state.Config.Zone, clients); err != nil {
			return dbus.NewError(dbusName+".Error", []interface{}{err.Error()})
		}
	}

	return nil
}

func loadState() (*WireWall, error) {
	state := new(WireWall)

	config, err := ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	state.Config = *config

	clients, err := readClients(clientsDir)
	if err != nil {
		return nil, err
	}
	state.Clients = clients

	return state, nil
}

func main() {
	state, err := loadState()
	if err != nil {
		panic(err)
	}

	if err := configureWG(state.Config.Interface, state.Clients); err != nil {
		panic(err)
	}

	if state.Config.Zone != nil {
		if err := updateDNS(*state.Config.Zone, state.Clients); err != nil {
			panic(err)
		}
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

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	conn.Export(state, dbusPath, dbusName)
	conn.Export(introspect.Introspectable(dbusIntro), dbusPath, "org.freedesktop.DBus.Introspectable")

	reply, err := conn.RequestName(dbusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		panic(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		panic("an instance of wirewalld is already running")
	}

	select {}
}
