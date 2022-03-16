// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"log"
	"reflect"
	"testing"
	"time"
)

func debounce(delay time.Duration, done chan bool, f func()) {
	var init bool
	for {
		select {
		case <-done:
			init = true
		case <-time.After(delay):
			if init {
				f()
				return
			}
		}
	}
}

const mockConnStartState = `:dummy.int NOTICE * :*** Looking up your hostname...
:dummy.int NOTICE * :*** Checking Ident
:dummy.int NOTICE * :*** Found your hostname
:dummy.int NOTICE * :*** No Ident response
:dummy.int 001 fhjones :Welcome to the DUMMY Internet Relay Chat Network fhjones
:dummy.int 005 fhjones NETWORK=DummyIRC NICKLEN=20 :are supported by this server
:dummy.int 375 fhjones :- dummy.int Message of the Day -
:dummy.int 372 fhjones :example motd
:dummy.int 376 fhjones :End of /MOTD command.
:fhjones!~user@local.int JOIN #channel * :realname
:dummy.int 332 fhjones #channel :example topic
:dummy.int 353 fhjones = #channel :fhjones!~user@local.int @nick2!nick2@other.int
:dummy.int 366 fhjones #channel :End of /NAMES list.
:dummy.int 354 fhjones 1 #channel ~user local.int fhjones 0 :realname
:dummy.int 354 fhjones 1 #channel nick2 other.int nick2 nick2 :realname2
:dummy.int 315 fhjones #channel :End of /WHO list.
:fhjones!~user@local.int JOIN #channel2 * :realname
:dummy.int 332 fhjones #channel2 :example topic
:dummy.int 353 fhjones = #channel2 :fhjones!~user@local.int @nick2!nick2@other.int
:dummy.int 366 fhjones #channel2 :End of /NAMES list.
:dummy.int 354 fhjones 1 #channel2 ~user local.int fhjones 0 :realname
:dummy.int 354 fhjones 1 #channel2 nick2 other.int nick2 nick2 :realname2
:dummy.int 315 fhjones #channel2 :End of /WHO list.
`

const mockConnEndState = `:nick2!nick2@other.int QUIT :example reason
:fhjones!~user@local.int PART #channel2 :example reason
:fhjones!~user@local.int NICK notjones
`

func TestState(t *testing.T) {
	c, conn, server := genMockConn(t)

	defer c.Close()
	go mockReadBuffer(conn)

	go func() {
		err := c.MockConnect(server)
		if err != nil {
			panic(err)
		}
	}()

	bounceStart := make(chan bool, 1)
	finishStart := make(chan bool, 1)
	go debounce(250*time.Millisecond, bounceStart, func() {
		if motd := c.ServerMOTD(); motd != "example motd" {
			t.Errorf("Client.ServerMOTD() returned invalid MOTD: %q", motd)
		}

		if network := c.NetworkName(); network != "DummyIRC" && network != "DUMMY" {
			t.Errorf("User.Network == %q, want \"DummyIRC\" or \"DUMMY\"", network)
		}

		if caseExample, ok := c.GetServerOption("NICKLEN"); !ok || caseExample != "20" {
			t.Errorf("Client.GetServerOptions returned invalid ISUPPORT variable: %q", caseExample)
		}

		t.Logf("getting user list")
		users := c.UserList()
		t.Logf("getting channel list")
		channels := c.ChannelList()

		if !reflect.DeepEqual(users, []string{"fhjones", "nick2"}) {
			// This could fail too, if sorting isn't occurring.
			t.Errorf("got state users %#v, wanted: %#v", users, []string{"fhjones", "nick2"})
		}

		if !reflect.DeepEqual(channels, []string{"#channel", "#channel2"}) {
			// This could fail too, if sorting isn't occurring.
			t.Errorf("got state channels %#v, wanted: %#v", channels, []string{"#channel", "#channel2"})
		}

		fullChannels := c.Channels()
		for i := 0; i < len(fullChannels); i++ {
			if fullChannels[i].Name != channels[i] {
				t.Errorf("fullChannels name doesn't map to same name in ChannelsList: %q :: %#v", fullChannels[i].Name, channels)
			}
		}

		fullUsers := c.Users()
		for i := 0; i < len(fullUsers); i++ {
			if fullUsers[i].Nick != users[i] {
				t.Errorf("fullUsers nick doesn't map to same nick in UsersList: %q :: %#v", fullUsers[i].Nick, users)
			}
		}

		ch := c.LookupChannel("#channel")
		if ch == nil {
			t.Error("Client.LookupChannel returned nil on existing channel")
			return
		}

		adm := ch.Admins(c)
		if adm == nil {
			t.Errorf("admin list is nil")
			t.Fail()
		}
		admList := []string{}
		for i := 0; i < len(adm); i++ {
			admList = append(admList, adm[i].Nick)
		}
		trusted := ch.Trusted(c)
		trustedList := []string{}
		for i := 0; i < len(trusted); i++ {
			trustedList = append(trustedList, trusted[i].Nick)
		}

		if !reflect.DeepEqual(admList, []string{"nick2"}) {
			t.Errorf("got Channel.Admins() == %#v, wanted %#v", admList, []string{"nick2"})
		}

		if !reflect.DeepEqual(trustedList, []string{"nick2"}) {
			t.Errorf("got Channel.Trusted() == %#v, wanted %#v", trustedList, []string{"nick2"})
		}

		if topic := ch.Topic; topic != "example topic" {
			t.Errorf("Channel.Topic == %q, want \"example topic\"", topic)
		}

		if ch.Network != "DummyIRC" && ch.Network != "DUMMY" {
			t.Errorf("Channel.Network == %q, want \"DummyIRC\" or \"DUMMY\"", ch.Network)
		}

		if in := ch.UserIn("fhjones"); !in {
			t.Errorf("Channel.UserIn == %t, want %t", in, true)
		}

		if users := ch.Users(c); len(users) != 2 {
			t.Errorf("Channel.Users == %#v, wanted length of 2", users)
		}

		if h := c.GetHost(); h != "local.int" {
			t.Errorf("Client.GetHost() == %q, want local.int", h)
		}

		if nick := c.GetNick(); nick != "fhjones" {
			t.Errorf("Client.GetNick() == %q, want nick", nick)
		}

		if ident := c.GetIdent(); ident != "~user" {
			t.Errorf("Client.GetIdent() == %q, want ~user", ident)
		}

		user := c.LookupUser("fhjones")
		if user == nil {
			t.Errorf("Client.LookupUser() returned nil on existing user")
			return
		}

		if !reflect.DeepEqual(user.ChannelList, []string{"#channel", "#channel2"}) {
			t.Errorf("User.ChannelList == %#v, wanted %#v", user.ChannelList, []string{"#channel", "#channel2"})
		}

		if count := len(user.Channels(c)); count != 2 {
			t.Errorf("len(User.Channels) == %d, want 2", count)
		}

		if user.Nick != "fhjones" {
			t.Errorf("User.Nick == %q, wanted \"nick\"", user.Nick)
		}

		if user.Extras.Name != "realname" {
			t.Errorf("User.Extras.Name == %q, wanted \"realname\"", user.Extras.Name)
		}

		if user.Host != "local.int" {
			t.Errorf("User.Host == %q, wanted \"local.int\"", user.Host)
		}

		if user.Ident != "~user" {
			t.Errorf("User.Ident == %q, wanted \"~user\"", user.Ident)
		}

		if user.Network != "DummyIRC" && user.Network != "DUMMY" {
			t.Errorf("User.Network == %q, want \"DummyIRC\" or \"DUMMY\"", user.Network)
		}

		if !user.InChannel("#channel2") {
			t.Error("User.InChannel() returned false for existing channel")
			return
		}

		finishStart <- true
	})

	cuid := c.Handlers.AddBg(UPDATE_STATE, func(c *Client, e Event) {
		println(e.String())
		bounceStart <- true
	})

	err := conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		log.Fatalf(err.Error())
	}
	_, err = conn.Write([]byte(mockConnStartState))
	if err != nil {
		panic(err)
	}

	select {
	case <-finishStart:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out while waiting for state update start")
	}
	c.Handlers.Remove(cuid)

	bounceEnd := make(chan bool, 1)
	finishEnd := make(chan bool, 1)
	go debounce(250*time.Millisecond, bounceEnd, func() {
		if !reflect.DeepEqual(c.ChannelList(), []string{"#channel"}) {
			t.Errorf("Client.ChannelList() == %#v, wanted %#v", c.ChannelList(), []string{"#channel"})
		}

		if !reflect.DeepEqual(c.UserList(), []string{"notjones"}) {
			t.Errorf("Client.UserList() == %#v, wanted %#v", c.UserList(), []string{"notjones"})
		}

		user := c.LookupUser("notjones")
		if user == nil {
			t.Fatal("Client.LookupUser() returned nil for existing user")
		}

		if !reflect.DeepEqual(user.ChannelList, []string{"#channel"}) {
			t.Errorf("user.ChannelList == %q, wanted %q", user.ChannelList, []string{"#channel"})
		}

		channel := c.LookupChannel("#channel")
		if channel == nil {
			t.Error("Client.LookupChannel() returned nil for existing channel")
		}

		if !reflect.DeepEqual(channel.UserList, []string{"notjones"}) {
			t.Errorf("channel.UserList == %q, wanted %q", channel.UserList, []string{"notjones"})
		}

		t.Logf(c.String())
		finishEnd <- true
	})

	cuid = c.Handlers.AddBg(UPDATE_STATE, func(c *Client, e Event) {
		bounceEnd <- true
	})

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(mockConnEndState))
	if err != nil {
		panic(err)
	}

	select {
	case <-finishEnd:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out while waiting for state update end")
	}
	c.Handlers.Remove(cuid)
}
