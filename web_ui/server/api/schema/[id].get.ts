import  fs from 'node:fs/promises';
import path from 'node:path';

const pb = useServerPocketbase();

export default defineEventHandler(
  async (event) => {
  
  const name = getRouterParam(event, "id");
  console.log(`Looking for block with name: ${name}`)

  const pbData = await pb.collection("blocks").getFirstListItem(`name="${name}"`);

  //const filepath = `/home/krbundy/GitHub/psaec/blocks/${name}.json`;
  //const data = await fs.readFile(filepath);
  
  const parsed_data: any = pbData.schema; //JSON.parse(data.toString());
  
  return parsed_data
});
