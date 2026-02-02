package tui

var (
	mascotPanda = `
 (^. .^)
 /| | \
  U  U
`
	mascotPandaWorking = `
 (O . O)
 /| | \
  U  U
`

	mascotWaifu1 = `
  ( ^_^)
  /|  |\
 ( (  ) )
  L    J
`
	mascotWaifu1Working = `
  ( o_o)
  /|  |\
 ( (  ) )
  L    J
`

	mascotWaifu2 = `
   ^__^
  (o.o )
  ( >< )
   "--"
`
	mascotWaifu2Working = `
   ^__^
  ( O.O)
  ( >< )
   "--"
`
)

func getMascot(typeStr string, working bool) string {
	switch typeStr {
	case "waifu1":
		if working {
			return mascotWaifu1Working
		}
		return mascotWaifu1
	case "waifu2":
		if working {
			return mascotWaifu2Working
		}
		return mascotWaifu2
	default: // panda
		if working {
			return mascotPandaWorking
		}
		return mascotPanda
	}
}
