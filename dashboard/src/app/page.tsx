import fs from "fs";
import path from "path";
import DashboardClient from "@/components/DashboardClient";

export default async function Page() {
  // Read the results directory relative to the project root
  const resultsDir = path.join(process.cwd(), "../results");
  let experiments: any[] = [];

  try {
    if (fs.existsSync(resultsDir)) {
      const files = fs.readdirSync(resultsDir);
      const jsonFiles = files.filter((f) => f.endsWith(".json"));

      for (const file of jsonFiles) {
        const filePath = path.join(resultsDir, file);
        const data = fs.readFileSync(filePath, "utf-8");
        experiments.push(JSON.parse(data));
      }
    }
  } catch (error) {
    console.error("Failed to read results directory:", error);
  }

  // Sort experiments by ID (timestamp)
  experiments.sort((a, b) => b.ExperimentID.localeCompare(a.ExperimentID));

  return <DashboardClient experiments={experiments} />;
}
