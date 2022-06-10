package accesslog

type options struct {
	filename     string
	requestBody  bool
	responseBody bool
	log          Logger
}

func defaultOptions() *options {
	return &options{
		requestBody:  true,
		responseBody: true,
	}
}

type Option func(opt *options)

func WithLogger(log Logger) Option {
	return func(opt *options) {
		opt.log = log
	}
}

func WithFileName(name string) Option {
	return func(opt *options) {
		opt.filename = name
	}
}

func WithRequestBody(t bool) Option {
	return func(opt *options) {
		opt.requestBody = t
	}
}

func WithResponseBody(t bool) Option {
	return func(opt *options) {
		opt.responseBody = t
	}
}
