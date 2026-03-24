<script setup lang="ts">
    const username = ref('');
    const passwd = ref('')
    //const errorMessage = ref('')

    
    const pb = usePB();

    const handleSubmit = async () => {
        console.log("Logging in!")

        try {

            const result = await pb.value.collection("users").authWithPassword(
                username.value, 
                passwd.value
            );
            if( result ) {
                await navigateTo({path: "/confirm"});
            }
        } catch (error) {

            
            
            
                console.log(error);
            }

    }
        

</script>


<template>
    <div class="items-center flex justify-center h-screen">
        <UCard class="max-w-md">
            <template #header>
                <p class="text-xl">Sign In</p>
            </template>
            
            <UInput v-model="username" class="p-4" variant="outline" placeholder="Enter your email"></UInput>
            <UInput v-model="passwd" class="p-4" type="password" variant="outline" placeholder="Enter your password"></UInput>
            
            <template #footer>
                <UButton @click="handleSubmit">Submit</UButton>
            </template>
        </UCard>
    </div>
</template>