package core

type NodeType string

const (
	NodeTypeMap    = "map"
	NodeTypeTask   = "task"
	NodeTypeReduce = "reduce"
)

type ExecutionNode struct {
	Id     int
	Type   NodeType
	TypeId string
}

type DAG struct {
	Nodes map[int]BlockInvocation
	Edges map[int][]int
}

func (g *DAG) AddNode(node ExecutionNode) error {
	return nil
}

func (g *DAG) ExpandNode(node ExecutionNode) error {
	return nil
}

// factory method
func NewDAGFromPipeline(p Pipeline) DAG {
	var g DAG

	return g

}
