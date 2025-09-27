package throttled

type CLI struct {
	Port      int `help:"Listen port number." required:"true"`
	CacheSize int `help:"LRU cache size." default:"100000"`
}
