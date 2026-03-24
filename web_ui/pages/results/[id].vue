<script setup lang="ts">

const route = useRoute();
const display = ref<string>("")

const pb = usePB();

const authData = await pb.value.collection("users").authRefresh();

// get the ID somehow
const targetId = route.params.id.toString() || "test";

const selectedFile = ref<string | null>(null);
const selectedFileId = ref<string | null>(null);
const showFile = ref<boolean>(false);
const showStandardOut = ref<boolean>(false)
const showStandardError = ref<boolean>(false)



const updateSelected = (name: string) => {
    selectedFile.value = name.toLowerCase();
}

const { data, refresh, status } = useAsyncData(`files_${route.fullPath}`, 
    async () => {
        //return await $fetch(`/api/results/${targetId}`)

        const data = await pb.value.collection("files").getList(
            1,50,
            {
                filter: `run_id="${route.params['id']}"`
            }
        );

        return data
    }
)

const runData = useAsyncData("run", 
    async () => {
        const data = await pb.value.collection("runs").getFirstListItem(`runId="${route.params['id']}"`);
        return data
    }
)

const runPipeline = async  () => {
/*
    const r = await pb.value.collection("runs").create(
        {
            run_id: targetId,
            pipeline: 
        }
    )
    const response = await $fetch("/api/pipeline/run", {
        method: "POST",
        body: {
            id: targetId
        }
    })

    console.log(response)
    display.value = JSON.stringify(response);
    */
   console.log("This function is no longer used.")
}

const selectFile = (file: string, id: string) => {
    showFile.value = false;
    selectedFile.value = file;
    selectedFileId.value = id;
    showStandardError.value = false;
    showStandardOut.value = false;
    showFile.value = true;
}

function downloadURI(uri: string, name: string) {
  var link = document.createElement("a");
  link.download = name;
  link.href = uri;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}

const downloadFile = async () => {
    console.log(selectedFile.value)
    if(selectedFileId.value != null){
        const dlId = selectedFileId.value;
        const record = await pb.value.collection("files").getOne(dlId);
        const fileToken = await pb.value.files.getToken();
        const url = pb.value.files.getUrl(record, record.data, {token: fileToken})
        downloadURI(url, record.id)
        
    }
}

const downloadAll = async () => {
    data.value?.items.forEach(
        async (record) => {
        const fileToken = await pb.value.files.getToken();
        const url = pb.value.files.getUrl(record, record.data, {token: fileToken})
        downloadURI(url, record.id)
        }
    )
}

const autoRefresh = async () => {
    setTimeout(
        () => {
            if(runData.data.value?.status !== "completed"){
                refresh()
                return autoRefresh()
            }
        },
        10000
    )
}

onMounted(
    () => {
        refresh()
        autoRefresh()
    }
)

const cleanupFileName = (baseName: string) => {
    return baseName.split("_")[0] + "." + baseName.split(".")[1]

}

const displayStdErr = () => {
    showFile.value = false;
    showStandardOut.value = false;
    showStandardError.value = true;

}

const displayStdOut = () => {
    showFile.value = false;
    showStandardOut.value = true;
    showStandardError.value = false;
}

</script>

<template>
  <div class="flex h-screen overflow-hidden">
    <aside class="w-64 p-4 border-r overflow-y-auto">
        <UCard class="mt-5">
            <p>Status: {{ runData.data.value?.status! }}</p>
            </UCard>
        <UCard class="mt-5">
            
            <UButtonGroup orientation="vertical" class="flex">
                <UButton
                class="bg-white text-slate-800 hover:bg-slate-200"
                @click="displayStdOut"
                > Standard Output</UButton>
                                <UButton
                class="bg-white text-slate-800 hover:bg-slate-200"
                @click="displayStdErr"
                > Standard Error</UButton>
            </UButtonGroup>
        </UCard>
        
        <UCard class="mt-5 text-centered">
            
            <UButtonGroup v-if="data" orientation="vertical" class="flex">
                <UButton v-for="file in data.items" 
                class="bg-white text-slate-800 hover:bg-slate-200"
                @click="() => selectFile(file.data, file.id)"
                > {{ cleanupFileName(file.data) }}</UButton>
            </UButtonGroup>
        </UCard>
        <UCard class="mt-5">
            
                <UButton @click="downloadFile" class="flex-grow">Download Selected</UButton>
            <p style="font-size: 0.6rem;">{{ selectedFile }}</p>
        </UCard>
   
        
    </aside>

    <main class="flex-1 p-6 overflow-y-auto">
            <UContainer v-if="showFile">
            <div v-if="selectedFile !== null && selectedFileId !== null">
                <UCard>
                    <div v-if="selectedFile.includes('.csv')">
                        <ResultsTable :id="selectedFileId" :name="selectedFile"></ResultsTable>
                    </div>
                    <div v-if="selectedFile.includes('.json')">
                        <ResultsJson :id="selectedFileId" :name="selectedFile"></ResultsJson>
                    </div>
                    <div v-if="selectedFile.includes('.geojson')">
                        <ResultsGeojson :id="selectedFileId" :name="selectedFile"></ResultsGeojson>
                    </div>
                    <div v-if="selectedFile.includes('.txt') || selectedFile.includes('.yaml') || selectedFile.includes('.toml')  || selectedFile.includes('.md')  ">
                        <ResultsText :id="selectedFileId" :name="selectedFile"></ResultsText>
                    </div>
                     <div v-if="selectedFile.includes('.png') || selectedFile.includes('.jpg') || selectedFile.includes('.webp')  || selectedFile.includes('.jpeg')  ">
                        <ResultsImage :id="selectedFileId" :name="selectedFile"></ResultsImage>
                    </div>
                        
                </UCard>
            </div>
        </UContainer>      
        <UContainer v-if="showStandardOut">
            <ResultsStd :id="targetId" :out="true"></ResultsStd>
        </UContainer>
        <UContainer v-if="showStandardError">
            <ResultsStd :id="targetId" :out="false"></ResultsStd>
        </UContainer>


    </main>
    </div>
</template>