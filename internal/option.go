package internal

import "sync"

type Option func(Owner)

func WithLogger(logger Logger) Option {
	return func(owner Owner) {
		owner.SetLogger(logger)
	}
}

func WithBytesBufferPool(bytesBufferPool *sync.Pool) Option {
	return func(owner Owner) {
		owner.SetBytesPool(bytesBufferPool)
	}
}
