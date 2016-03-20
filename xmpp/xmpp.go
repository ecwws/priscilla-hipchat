package xmpp

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
)

const (
	NsStream  = "http://etherx.jabber.org/streams"
	NsTLS     = "urn:ietf:params:xml:ns:xmpp-tls"
	NsHipchat = "http://hipchat.com"

	streamStart = `<stream:stream
		xmlns='jabber:client'
		xmlns:stream='http://etherx.jabber.org/streams'
		from='%s'
		to='%s'
		version='1.0'>`
	streamEnd = "</stream:stream>"
)

type Conn struct {
	raw     net.Conn
	decoder *xml.Decoder
	encoder *xml.Encoder
}

type emptyElement struct {
	XMLName xml.Name
}

type charElement struct {
	XMLName xml.Name
	Value   string `xml:,chardata`
}

type required struct{}

type features struct {
	XMLName    xml.Name  `xml:"features"`
	StartTLS   *required `xml:"starttls>required"`
	Mechanisms []string  `xml:"mechanisms>mechanism"`
}

type authResponse struct {
	XMLName  xml.Name `xml:"success"`
	Jid      string   `xml:"jid,attr"`
	ApiHost  string   `xml:"api_host,attr"`
	ChatHost string   `xml:"chat_host,attr"`
	MucHost  string   `xml:"muc_host,attr"`
	WebHost  string   `xml:"web_host,attr"`
}

type xmppIq struct {
	XMLName xml.Name `xml:"iq"`
	Type    string   `xml:"type,attr"`
	Id      string   `xml:"id,attr"`
	Query   interface{}
}

type xmppPresence struct {
	XMLName xml.Name `xml:"presence"`
	From    string   `xml:"from,attr"`
	Status  interface{}
}

type xmppAuth struct {
	XMLName xml.Name `xml:"auth"`
	Ns      string   `xml:"xmlns,attr"`
	Value   string   `xml:",chardata"`
}

type xmppShow struct {
	XMLName xml.Name `xml:"show"`
	Value   string   `xml:",chardata"`
}

func Connect(host string) (*Conn, error) {
	c := new(Conn)

	conn, err := net.Dial("tcp", host+":5222")

	if err != nil {
		return c, err
	}

	c.raw = conn
	c.decoder = xml.NewDecoder(c.raw)
	c.encoder = xml.NewEncoder(c.raw)

	return c, nil
}

func (c *Conn) StreamStart(id, host string) {
	fmt.Fprintf(c.raw, streamStart, id, host)
}

func (c *Conn) RecvNext() (element xml.StartElement, err error) {
	for {
		var t xml.Token
		t, err = c.decoder.Token()
		if err != nil {
			return element, err
		}

		switch t := t.(type) {
		case xml.StartElement:
			element = t
			if element.Name.Local == "" {
				err = errors.New("Bad XML response")
				return
			}

			return
		}
	}
}

func (c *Conn) RecvFeatures() *features {
	var f features
	c.decoder.DecodeElement(&f, nil)
	return &f
}

func (c *Conn) StartTLS() {
	starttls := emptyElement{
		XMLName: xml.Name{Local: "starttls", Space: NsTLS},
	}
	c.encoder.Encode(starttls)
}

func (c *Conn) UseTLS(host string) {
	c.raw = tls.Client(c.raw, &tls.Config{ServerName: host})
	c.decoder = xml.NewDecoder(c.raw)
	c.encoder = xml.NewEncoder(c.raw)
}

func (c *Conn) Auth(username, password, resource string) (*authResponse, error) {
	token := []byte{'\x00'}

	token = append(token, []byte(username)...)
	token = append(token, '\x00')
	token = append(token, []byte(password)...)
	token = append(token, '\x00')
	token = append(token, []byte(resource)...)

	encodedToken := base64.StdEncoding.EncodeToString(token)

	auth := xmppAuth{
		Ns:    NsHipchat,
		Value: encodedToken,
	}
	// out, _ := xml.Marshal(auth)
	// fmt.Println(string(out))
	c.encoder.Encode(auth)

	var response authResponse

	err := c.decoder.Decode(&response)

	return &response, err
}

func id() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("%x", b)
}

func (c *Conn) Available(jid string) {
	available := xmppPresence{
		From:   jid,
		Status: &xmppShow{Value: "chat"},
	}

	c.encoder.Encode(available)
}

func (c *Conn) KeepAlive() {
	fmt.Fprintf(c.raw, " ")
}

func (c *Conn) ReadRaw() {
	for {
		buf := make([]byte, 128)
		count, _ := c.raw.Read(buf)

		fmt.Print(string(buf[:count]))
	}
}