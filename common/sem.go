package common

type Sem chan struct{}

func (s Sem) Acquare() bool {
	s <- struct{}{}
	return true
}

func (s Sem) Release() bool {
	<-s
	return false
}
