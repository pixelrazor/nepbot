package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "embed"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

const (
	discordTokenKey = "DISCORD_TOKEN"
)

//go:embed ssd.dca
var ssd []byte

func main() {
	discordToken, ok := os.LookupEnv(discordTokenKey)
	if !ok {
		log.Fatalln("Missing DISCORD_TOKEN")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalln("Failed to create dg session:", err)
	}

	bot := NepBot{
		s: dg,
	}

	bot.handlers()

	if err := bot.run(); err != nil {
		log.Fatalln("Failed to start bot:", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("peace out")

	/*for {
		<-time.After(10 * time.Second)
		fmt.Println("gonna talk")
		vc, err := dg.ChannelVoiceJoin("159526587681734657", "159526588554280960", false, true)
		if err != nil {
			panic(err)
		}
		fmt.Println("set speaking true:", vc.Speaking(true))
		dec := dca.NewDecoder(bytes.NewReader(ssd))
		done := make(chan error)
		dca.NewStream(dec, vc, done)
		err = <-done
		fmt.Println("done:", err)
		fmt.Println("set speaking false:", vc.Speaking(false))
		fmt.Println("speaking disconnect:", vc.Disconnect())

	}*/

}

type NepBot struct {
	s *discordgo.Session
}

func (nb *NepBot) run() error {
	if err := nb.s.Open(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	return nil
}

func (nb *NepBot) handlers() {
	nb.s.AddHandler(nb.onReady())
	nb.s.AddHandler(nb.onVoiceStateUpdate())
	nb.s.Identify.Intents = discordgo.IntentGuildVoiceStates
}

func (nb *NepBot) onReady() interface{} {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("Nep reporting for duty!")
	}
}

func (nb *NepBot) onVoiceStateUpdate() interface{} {
	var lock sync.Mutex
	ssdServers := make(map[string]chan struct{})
	return func(s *discordgo.Session, vsu *discordgo.VoiceStateUpdate) {
		if vsu.UserID == s.State.User.ID {
			return
		}
		afkChannel := ""
		guild, err := Guild(s, vsu.GuildID)
		if err != nil {
			log.Printf("Guild %v state fetch error: %v; assuming no afk channel\n", vsu.GuildID, err)
		} else {
			afkChannel = guild.AfkChannelID
		}

		if vsu.ChannelID == "" || vsu.ChannelID == afkChannel {
			return
		} else if vsu.BeforeUpdate != nil && vsu.ChannelID == vsu.BeforeUpdate.ChannelID {
			return
		}

		var waitMyTurn chan struct{}
		func() {
			lock.Lock()
			defer lock.Unlock()
			if _, ok := ssdServers[vsu.GuildID]; !ok {
				ssdServers[vsu.GuildID] = make(chan struct{}, 1)
			}
			waitMyTurn = ssdServers[vsu.GuildID]
		}()
		waitMyTurn <- struct{}{}
		defer func() {
			<-waitMyTurn
		}()
		err = nb.playSsd(vsu.GuildID, vsu.ChannelID)
		if err != nil {
			log.Printf("Failed playing ssd for guild %v channel %v: %v\n", vsu.GuildID, vsu.ChannelID, err)
		}
	}
}

func (nb *NepBot) playSsd(guild, channel string) error {
	vc, err := nb.s.ChannelVoiceJoin(guild, channel, false, true)
	if err != nil {
		return fmt.Errorf("failed to join vc: %w", err)
	}
	defer time.Sleep(200 * time.Millisecond)
	defer vc.Disconnect()
	if err := vc.Speaking(true); err != nil {
		return fmt.Errorf("failed to start speaking: %w", err)
	}
	defer vc.Speaking(false)
	dec := dca.NewDecoder(bytes.NewReader(ssd))
	done := make(chan error)
	dca.NewStream(dec, vc, done)
	fmt.Println("nep gonna nep")
	if err := <-done; err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("error while streaming ssd: %w", err)
	}
	return nil
}

func Guild(s *discordgo.Session, guildID string) (*discordgo.Guild, error) {
	guild, err := s.State.Guild(guildID)
	if err != nil || guild.OwnerID == "" { // For some reason ownerID isn't set when first cached? The afk channel isn't either, but "" is valid.
		guild, err := s.Guild(guildID)
		if err != nil {
			return nil, err
		}
		s.State.GuildAdd(guild)
		return guild, nil
	}
	return guild, nil
}
