package accesslog

type Options struct {
	cfg * Conf
}

type Option func(opt * Options)

func LoggerOption(cfg * Conf) Option {
	return func(opt *Options) {
		opt.cfg = cfg
	}
}
