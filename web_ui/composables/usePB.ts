
import PocketBase from "pocketbase";

const pb = new PocketBase("https://acg-floating-204-197-5-169.acg.maine.edu/");


export default function() {
    return useState("pb", () => pb);
}