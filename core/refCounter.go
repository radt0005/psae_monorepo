package core

import "github.com/google/uuid"

type ReferenceCounter struct {
	references map[uuid.UUID]uint
}

func (r *ReferenceCounter) Add(id uuid.UUID) {

	refs, ok := r.references[id]

	if !ok {
		r.references[id] = 1
	} else {
		r.references[id] = refs + 1
	}

}

func (r *ReferenceCounter) Remove(id uuid.UUID) {
	refs, ok := r.references[id]
	if ok {
		r.references[id] = refs - 1
	} else {
		r.references[id] = 0
	}

}

func (r *ReferenceCounter) GetReferenceCount(id uuid.UUID) int {
	refs, ok := r.references[id]
	if !ok {
		return 0
	} else {
		return int(refs)
	}
}

func (r *ReferenceCounter) GetUnreferencedItems() []uuid.UUID {
	var ids []uuid.UUID

	for key, value := range r.references {

		if value <= 0 {
			ids = append(ids, key)
		}
	}

	return ids
}
