package core

import (
	"os"
	"path"

	"github.com/google/uuid"
)

func CreateBlockDirectory(id uuid.UUID, workdir string) error {

	p := path.Join(workdir, id.String())
	err := os.MkdirAll(p, os.ModeDir)

	return err
}

func CollectOuptuts(id uuid.UUID) ([]string, error) {

	return []string{}, nil
}

func LoadBlockInfo() {

}

func SaveBlockInfo(block Block) error {
	return nil
}
