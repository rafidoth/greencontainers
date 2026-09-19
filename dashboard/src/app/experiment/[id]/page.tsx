import fs from "fs";
import path from "path";
import Link from "next/link";
import { ArrowLeft, Activity, Zap, Cpu, MemoryStick, Clock } from "lucide-react";

export default async function ExperimentPage({
  params,
}: {
  params: { id: string };
}) {
  const { id } = await params; // Important in Next.js 14+: params should be awaited if necessary, wait, let's keep it sync for older versions or just use it.
  
  const resultsDir = path.join(process.cwd(), "../results");
  const filePath = path.join(resultsDir, `${id}.json`);

  let data: any = null;

  try {
    if (fs.existsSync(filePath)) {
      const fileContent = fs.readFileSync(filePath, "utf-8");
      data = JSON.parse(fileContent);
    }
  } catch (error) {
    console.error("Failed to read experiment file:", error);
  }

  if (!data) {
    return (
      <div className="min-h-screen bg-gray-50 flex flex-col items-center justify-center p-8">
        <h1 className="text-2xl font-bold text-gray-800 mb-4">Experiment not found</h1>
        <Link href="/" className="text-blue-600 hover:underline flex items-center gap-2">
          <ArrowLeft className="w-4 h-4" /> Back to Dashboard
        </Link>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 font-sans p-8">
      <div className="max-w-6xl mx-auto">
        <Link href="/" className="inline-flex items-center gap-2 text-blue-600 hover:underline mb-6">
          <ArrowLeft className="w-4 h-4" /> Back to Dashboard
        </Link>

        <header className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900 flex items-center gap-3">
            <Activity className="text-blue-500 w-8 h-8" />
            Experiment: {data.ExperimentID}
          </h1>
        </header>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          {/* Config Summary */}
          <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
            <h2 className="text-xl font-semibold mb-4 border-b pb-2">Configuration</h2>
            <dl className="grid grid-cols-2 gap-4 text-sm">
              <dt className="text-gray-500">Base Image</dt>
              <dd className="font-medium">{data.Config?.Build?.BaseImage}</dd>
              <dt className="text-gray-500">Context</dt>
              <dd className="font-medium">{data.Config?.Build?.Context}</dd>
              <dt className="text-gray-500">Trials</dt>
              <dd className="font-medium">{data.Config?.Trials}</dd>
              <dt className="text-gray-500">Host Port</dt>
              <dd className="font-medium">{data.Config?.Container?.HostPort}</dd>
            </dl>
          </div>

          {/* Baseline Summary */}
          <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
            <h2 className="text-xl font-semibold mb-4 border-b pb-2">Baseline</h2>
            <dl className="grid grid-cols-2 gap-4 text-sm">
              <dt className="text-gray-500">Power (Watts)</dt>
              <dd className="font-medium">{(data.Baseline?.PowerWatts || 0).toFixed(2)} W</dd>
              <dt className="text-gray-500">Energy (Joules)</dt>
              <dd className="font-medium">{(data.Baseline?.EnergyJoules || 0).toFixed(2)} J</dd>
              <dt className="text-gray-500">Duration</dt>
              <dd className="font-medium">{data.Config?.Baseline?.Duration}</dd>
              <dt className="text-gray-500">CPU Usage</dt>
              <dd className="font-medium">{(data.Baseline?.CPUAvgPercent || 0).toFixed(2)} %</dd>
            </dl>
          </div>
        </div>

        {/* Stages Table */}
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden mb-8">
          <div className="p-6 border-b border-gray-200">
            <h2 className="text-xl font-semibold">Stage Results</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-gray-500 bg-gray-50 uppercase">
                <tr>
                  <th className="px-6 py-4">Trial</th>
                  <th className="px-6 py-4">Stage</th>
                  <th className="px-6 py-4">Duration (s)</th>
                  <th className="px-6 py-4">Energy (J)</th>
                  <th className="px-6 py-4">Avg Power (W)</th>
                  <th className="px-6 py-4">CPU (%)</th>
                  <th className="px-6 py-4">Mem (MB)</th>
                  <th className="px-6 py-4">Latency (ms)</th>
                </tr>
              </thead>
              <tbody>
                {data.Stages?.map((stage: any, i: number) => (
                  <tr key={i} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-6 py-4 font-medium text-gray-700">{stage.TrialNumber}</td>
                    <td className="px-6 py-4">
                      <span className="px-2.5 py-1 rounded-full bg-blue-50 text-blue-700 text-xs font-medium border border-blue-100">
                        {stage.StageName}
                      </span>
                    </td>
                    <td className="px-6 py-4">{(stage.Duration / 1e9).toFixed(2)}</td>
                    <td className="px-6 py-4 font-medium text-green-600">
                      {(stage.EnergyCorrectedJ ?? stage.RawEnergyJoules ?? 0).toFixed(2)}
                    </td>
                    <td className="px-6 py-4 text-orange-600">
                      {(stage.AveragePowerWatts || 0).toFixed(2)}
                    </td>
                    <td className="px-6 py-4">{(stage.CPUAvgPercent || 0).toFixed(2)}</td>
                    <td className="px-6 py-4">{(stage.MemAvgMB || 0).toFixed(2)}</td>
                    <td className="px-6 py-4">
                      {stage.K6Result ? stage.K6Result.avg_latency_ms.toFixed(2) : "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

      </div>
    </div>
  );
}
