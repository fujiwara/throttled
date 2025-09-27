package throttled

type CLI struct {
	Port      int    `help:"Listen port number." required:"true"`
	CacheSize int    `help:"LRU cache size." default:"100000"`
	LogLevel  string `help:"Log level (debug, info, warn, error)." default:"info"`
}
