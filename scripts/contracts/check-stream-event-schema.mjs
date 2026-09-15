import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const root = process.cwd();
const schemaPath = path.join(
  root,
  "packages/contracts/schemas/stream-event.v1.schema.json",
);
const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));
const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: true,
});
addFormats(ajv);
ajv.compile(schema);
console.log("public StreamEvent schema strict compile: PASS");
