package utils

func Map[T, O any](in []T, mapper func(T) O) []O {
	out := make([]O, len(in))
	for i, v := range in {
		out[i] = mapper(v)
	}
	return out
}
