<script setup lang="ts">


type RunHistoryItem = {
    id: string,
    created: string, 
    status: string,
    updated: string
    url: string
}

const columns = [
    {
        header: "ID",
        accessorKey: "id",
        key: "id"
    },
    {
        header: "Created",
        accessorKey: "created",
        key: "created"
    },
    {
        header: "Last Updated",
        accessorKey: "updated",
        key: "updated"
    },
    {
        header: "Status",
        accessorKey: "status",
        key: "status"
    },
    {
        header: "Edit",
        key: "url"
    }
]

const history = ref<RunHistoryItem[]>([]);
// get the route params
const temp = ref<any>("")

const pb = usePB();

const getUserRuns = async () => {
    const authData = await pb.value.collection('users').authRefresh();
    const userId = authData.record.id;
    const response = await pb.value.collection("runs").getList(
        1,50,
        {
            filter: `userId="${userId}"`,
            sort: "-created"
        }
    );
    return response
}

onMounted(
    async () => {
        const response = await getUserRuns();
        console.log(response);
        temp.value = response.items
        if(response.items){

            history.value = response.items.map(
                (item) => {
                    return {
                        id: item.runId,
                        created: item.created.toString(),
                        updated: item.updated,
                        status: item.status,
                        url: `results/${item.runId}`
                    } as RunHistoryItem
                }
            )
        }
    }
)

function onSelect(row: any, e?: Event) {
  /* If you decide to also select the column you can do this  */
  //row.toggleSelected(!row.getIsSelected())

  console.log(e)

  navigateTo(row.url)
}


</script>

<template>
    <UContainer>
        <p>My Runs</p>
        <div v-if="history.length > 0">
            <UTable :rows="history" :columns="columns" @select="onSelect">
                <template #edit-cell="{ row }">
                    <UButton :to="row.url">View</UButton>
                </template>

            </UTable>
        </div>

    </UContainer>
</template>