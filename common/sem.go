package common

type Sem chan struct{}

func (s Sem) Acquare() {
	s <- struct{}{}
}

func (s Sem) Release() {
	<-s
}
