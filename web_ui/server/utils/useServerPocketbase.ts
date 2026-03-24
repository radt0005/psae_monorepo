import PocketBase from "pocketbase";

export const useServerPocketbase = () => {
        const pb = new PocketBase(process.env.POCKETBASE_URL);
        pb.admins.authWithPassword(
            process.env.POCKETBASE_USER!,
            process.env.POCKETBASE_PASSWORD!
        )
        

        return pb
    }