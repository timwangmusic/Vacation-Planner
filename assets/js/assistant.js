import OpenAI from "openai";
const openai = new OpenAI();


async function main() {
    const assistant = await openai.beta.assistants.create(
        {
            name: "Hermes",
            instructions: "I am the god of travel, and I would help you plan your travel",
            tools:[{
                type: "file_search"
            }],
        }
    );
}

main();