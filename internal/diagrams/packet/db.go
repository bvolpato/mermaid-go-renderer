// Ported from mermaid 12.2.0, diagrams/packet/db.ts and types.ts (MIT).

package packet

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// Block is Required<PacketBlock>: one block of a row.
type Block struct {
	Start float64
	End   float64
	Bits  float64
	Label string
}

// PacketWord is one row of blocks.
type PacketWord []Block

// DB is PacketDB.
type DB struct {
	render.CommonDB
	ctx    *render.Context
	packet []PacketWord
}

func newDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, packet: []PacketWord{}}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetConfig is getConfig(): the default config of the packet diagram with
// the config of the render on top.
func (db *DB) GetConfig() *js.Obj[any] {
	spread := js.NewObj[any]()
	js.AsObj(config.DefaultSection("packet")).Each(func(key string, v any) { spread.Set(key, v) })
	if user := js.AsObj(db.ctx.Config.Get("packet")); user != nil {
		user.Each(func(key string, v any) { spread.Set(key, v) })
	}
	cfg := utils.CleanAndMerge(spread, nil)
	if js.Truthy(cfg.Get("showBits")) {
		cfg.Set("paddingY", js.Add(cfg.Get("paddingY"), 10.0))
	}
	return cfg
}

// GetPacket is getPacket().
func (db *DB) GetPacket() []PacketWord { return db.packet }

// PushWord is pushWord(word).
func (db *DB) PushWord(word PacketWord) {
	if len(word) > 0 {
		db.packet = append(db.packet, word)
	}
}
