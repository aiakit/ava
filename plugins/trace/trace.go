package trace

type Noop struct{}

func (n *Noop) Carrier() {
	return
}

func NewNoop() *Noop {
	return new(Noop)
}

func (n *Noop) Name() string {
	return "noop"
}

func (n *Noop) Finish() {
	return
}

func (n *Noop) TraceId() string {
	return ""
}

// Span
// |---TraceId:1     ----->RPC----->       |---TraceId:1
//       |---ParentSpanId:0                            |---ParentSpanId:222
//           |---SpanId:222                                 |---SpanId:223

// Span this is a demo,need to be richer
type Span struct {
	SpanId       uint32
	ParentSpanId int32
	traceId      string
}

func (s *Span) Carrier() {
	s.ParentSpanId += 1
	s.SpanId += 1
}

func (s *Span) Finish() {
	// todo buffer flush to cloud or something
	return
}

func (s *Span) Name() string {
	return "span"
}

func (s *Span) TraceId() string {
	return s.traceId
}

func NewSpan() *Span {
	return &Span{
		traceId:      "",
		ParentSpanId: -1,
		SpanId:       1,
	}
}
