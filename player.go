package main

import (
	"io"
	"log"
	"net"
	"bytes"
)

const (
	chunkRadius = 10
)

type Player struct {
	Entity
	game         *Game
	conn         net.Conn
	position     XYZ
	orientation  Orientation
	txQueue      chan []byte
}

func StartPlayer(game *Game, conn net.Conn) {
	player := &Player{
		game:        game,
		conn:        conn,
		position:    StartPosition,
		orientation: Orientation{0, 0},
		txQueue:     make(chan []byte, 128),
	}

	go player.ReceiveLoop()
	go player.TransmitLoop()

	game.Enqueue(func(game *Game) {
		game.AddPlayer(player)
		player.postLogin()
	})
}

func (player *Player) PacketKeepAlive() {
}

func (player *Player) PacketChatMessage(message string) {
	log.Stderrf("PacketChatMessage message=%s", message)
}

func (player *Player) PacketFlying(flying bool) {
}

func (player *Player) PacketPlayerPosition(position *XYZ, stance float64, flying bool) {
	log.Stderrf("PacketPlayerPosition position=(%.2f, %.2f, %.2f) stance=%.2f flying=%v",
		position.x, position.y, position.z, stance, flying)
}

func (player *Player) PacketPlayerLook(orientation *Orientation, flying bool) {
	log.Stderrf("PacketPlayerLook orientation=(%.2f, %.2f) flying=%v",
		orientation.rotation, orientation.pitch, flying)
}

func (player *Player) PacketPlayerDigging(status byte, x int32, y byte, z int32, face byte) {
	log.Stderrf("PacketPlayerDigging status=%d x=%d y=%d z=%d face=%d",
		status, x, y, z, face)
}

func (player *Player) PacketPlayerBlockPlacement(id int16, x int32, y byte, z int32, direction byte) {
	log.Stderrf("PacketPlayerBlockPlacement id=%d x=%d y=%d z=%d direction=%d",
		id, x, y, z, direction)
}

func (player *Player) PacketDisconnect(reason string) {
	log.Stderrf("PacketDisconnect reason=%s", reason)
	player.game.Enqueue(func(game *Game) { game.RemovePlayer(player) })
}

func (player *Player) ReceiveLoop() {
	for {
		err := ReadPacket(player.conn, player)
		if err != nil {
			log.Stderr("ReceiveLoop failed: ", err.String())
			return
		}
	}
}

func (player *Player) TransmitLoop() {
	for {
		bs := <-player.txQueue
		_, err := player.conn.Write(bs)
		if err != nil {
			log.Stderr("TransmitLoop failed: ", err.String())
			return
		}
	}
}

func (player *Player) sendChunks(writer io.Writer) {
	playerX := int32(player.position.x) / ChunkSizeX
	playerZ := int32(player.position.z) / ChunkSizeZ

	for z := playerZ - chunkRadius; z < playerZ + chunkRadius; z++ {
		for x := playerX - chunkRadius; x < playerX + chunkRadius; x++ {
			WritePreChunk(writer, x, z, true)
		}
	}

	for z := playerZ - chunkRadius; z < playerZ + chunkRadius; z++ {
		for x := playerX - chunkRadius; x < playerX + chunkRadius; x++ {
			chunk := player.game.chunkManager.Get(x, z)
			WriteMapChunk(writer, chunk)
		}
	}
}

func (player *Player) postLogin() {
	buf := &bytes.Buffer{}
	WriteSpawnPosition(buf, &player.position)
	player.sendChunks(buf)
	WritePlayerInventory(buf)
	WritePlayerPositionLook(buf, &player.position, &player.orientation,
		0, false)
	player.txQueue <- buf.Bytes()
}

func (player *Player) SendTimeUpdate(time int64) {
	buf := &bytes.Buffer{}
	WriteTimeUpdate(buf, time)
	player.txQueue <- buf.Bytes()
}