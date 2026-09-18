"use client";

import React, { useState } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import { Activity, Zap, Cpu, MemoryStick, Box } from "lucide-react";

export default function DashboardClient({ experiments }: { experiments: any[] }) {
  const [selectedMetric, setSelectedMetric] = useState("EnergyCorrectedJ");

  if (experiments.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-gray-500">
        <Activity className="w-12 h-12 mb-4 text-gray-300" />
        <p className="text-lg">No experiment results found in ./results directory.</p>
      </div>
    );
  }

  // Flatten stages for charting
  const flatStages = experiments.flatMap((exp) =>
    exp.Stages.map((stage: any) => ({
      ...stage,
      baseImage: stage.BaseImage || exp.Config?.Build?.BaseImage || "unknown",
      stageName: stage.StageName || stage.Stage,
      trialNum: stage.TrialNumber,
      energy: stage.EnergyCorrectedJ || stage.RawEnergyJoules,
      power: stage.AveragePowerWatts,
      cpu: stage.CPUAvgPercent,
      memory: stage.MemAvgMB,
      latency: stage.K6Result ? stage.K6Result.avg_latency_ms : 0,
      throughput: stage.K6Result ? stage.K6Result.throughput_rps : 0,
    }))
  );

  const LIFECYCLE_TYPES = ["build", "start", "stop", "cleanup", "pull"];

  // Group by Base Image and Stage Name to get averages across trials
  const aggregateData = (isLifecycle: boolean) => {
    const map = new Map<string, any>();

    flatStages.forEach((s: any) => {
      const isCurrentLifecycle = LIFECYCLE_TYPES.includes(s.StageType);
      if (isLifecycle !== isCurrentLifecycle) return;

      const key = `${s.baseImage}-${s.stageName}`;
      if (!map.has(key)) {
        map.set(key, {
          name: s.stageName,
          baseImage: s.baseImage,
          count: 0,
          energy: 0,
          power: 0,
          cpu: 0,
          memory: 0,
          latency: 0,
          throughput: 0,
        });
      }

      const entry = map.get(key);
      entry.count += 1;
      entry.energy += s.energy;
      entry.power += s.power;
      entry.cpu += s.cpu;
      entry.memory += s.memory;
      entry.latency += s.latency;
      entry.throughput += s.throughput;
    });

    const results: any[] = [];
    map.forEach((value) => {
      results.push({
        name: value.name,
        baseImage: value.baseImage,
        energy: value.energy / value.count,
        power: value.power / value.count,
        cpu: value.cpu / value.count,
        memory: value.memory / value.count,
        latency: value.latency / value.count,
        throughput: value.throughput / value.count,
      });
    });

    // Reformat for Recharts (grouping by stage, comparing images)
    const chartDataMap = new Map<string, any>();
    results.forEach((r) => {
      if (!chartDataMap.has(r.name)) {
        chartDataMap.set(r.name, { stage: r.name });
      }
      const entry = chartDataMap.get(r.name);
      entry[`${r.baseImage}_energy`] = r.energy;
      entry[`${r.baseImage}_power`] = r.power;
      entry[`${r.baseImage}_cpu`] = r.cpu;
      entry[`${r.baseImage}_memory`] = r.memory;
      entry[`${r.baseImage}_latency`] = r.latency;
      entry[`${r.baseImage}_throughput`] = r.throughput;
    });

    return Array.from(chartDataMap.values());
  };

  const workloadData = aggregateData(false);
  const lifecycleData = aggregateData(true);
  const baseImages = Array.from(new Set(flatStages.map((s: any) => s.baseImage)));
  const colors = ["#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6"];

  const metrics = [
    { id: "energy", label: "Energy (Joules)", icon: Zap },
    { id: "power", label: "Avg Power (Watts)", icon: Zap },
    { id: "cpu", label: "CPU Usage (%)", icon: Cpu },
    { id: "memory", label: "Memory (MB)", icon: MemoryStick },
  ];

  return (
    <div className="min-h-screen bg-gray-50 text-gray-900 font-sans p-8">
      <div className="max-w-6xl mx-auto">
        <header className="mb-8 flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 flex items-center gap-3">
              <Zap className="text-green-500 w-8 h-8" />
              Green Containers Dashboard
            </h1>
            <p className="text-gray-500 mt-2">
              Showing aggregated results across {experiments.length} experiments and {flatStages.length} stages.
            </p>
          </div>
        </header>

        {/* Metric Selector */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {metrics.map((m) => {
            const Icon = m.icon;
            const isSelected = selectedMetric === m.id;
            return (
              <button
                key={m.id}
                onClick={() => setSelectedMetric(m.id)}
                className={`p-4 rounded-xl border flex flex-col items-center justify-center gap-2 transition-all ${
                  isSelected
                    ? "bg-white border-green-500 shadow-sm ring-1 ring-green-500 text-green-700"
                    : "bg-white border-gray-200 text-gray-600 hover:border-green-300 hover:bg-green-50"
                }`}
              >
                <Icon className={`w-6 h-6 ${isSelected ? "text-green-500" : "text-gray-400"}`} />
                <span className="text-sm font-medium text-center">{m.label}</span>
              </button>
            );
          })}
        </div>

        <div className="flex flex-col gap-8 mb-8">
          {/* Workload Chart */}
          <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
            <h2 className="text-xl font-semibold mb-6 flex items-center gap-2">
              <Activity className="w-5 h-5 text-blue-500" />
              Workload Performance
            </h2>
            <div className="h-80 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={workloadData} margin={{ top: 20, right: 10, left: 0, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e5e7eb" />
                  <XAxis dataKey="stage" axisLine={false} tickLine={false} tick={{ fill: "#6b7280" }} dy={10} />
                  <YAxis axisLine={false} tickLine={false} tick={{ fill: "#6b7280" }} />
                  <Tooltip cursor={{ fill: "#f3f4f6" }} contentStyle={{ borderRadius: "8px", border: "none", boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)" }} />
                  <Legend wrapperStyle={{ paddingTop: "20px" }} />
                  {baseImages.map((img, i) => (
                    <Bar key={img} dataKey={`${img}_${selectedMetric}`} name={img as string} fill={colors[i % colors.length]} radius={[4, 4, 0, 0]} maxBarSize={40} />
                  ))}
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          {/* Lifecycle Chart */}
          <div className="bg-white p-6 rounded-2xl border border-gray-200 shadow-sm">
            <h2 className="text-xl font-semibold mb-6 flex items-center gap-2">
              <Box className="w-5 h-5 text-orange-500" />
              Container Lifecycle Overhead
            </h2>
            <div className="h-80 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={lifecycleData} margin={{ top: 20, right: 10, left: 0, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e5e7eb" />
                  <XAxis dataKey="stage" axisLine={false} tickLine={false} tick={{ fill: "#6b7280" }} dy={10} />
                  <YAxis axisLine={false} tickLine={false} tick={{ fill: "#6b7280" }} />
                  <Tooltip cursor={{ fill: "#f3f4f6" }} contentStyle={{ borderRadius: "8px", border: "none", boxShadow: "0 4px 6px -1px rgb(0 0 0 / 0.1)" }} />
                  <Legend wrapperStyle={{ paddingTop: "20px" }} />
                  {baseImages.map((img, i) => (
                    <Bar key={img} dataKey={`${img}_${selectedMetric}`} name={img as string} fill={colors[i % colors.length]} radius={[4, 4, 0, 0]} maxBarSize={40} />
                  ))}
                </BarChart>
              </ResponsiveContainer>
            </div>
            <p className="text-xs text-gray-400 mt-4 text-center">Note: Lifecycle events happen very fast, so energy (Joules) will appear much lower than 60-second load tests.</p>
          </div>
        </div>

        {/* Raw Data Table */}
        <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
          <div className="p-6 border-b border-gray-200">
            <h2 className="text-xl font-semibold">Latest Experiments</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-gray-500 bg-gray-50 uppercase">
                <tr>
                  <th className="px-6 py-4">Experiment</th>
                  <th className="px-6 py-4">Base Image</th>
                  <th className="px-6 py-4">Trials</th>
                  <th className="px-6 py-4">Stages</th>
                  <th className="px-6 py-4">Baseline Power</th>
                </tr>
              </thead>
              <tbody>
                {experiments.map((exp, i) => (
                  <tr key={i} className="border-b border-gray-100 hover:bg-gray-50">
                    <td className="px-6 py-4 font-medium">{exp.ExperimentID}</td>
                    <td className="px-6 py-4">
                      <span className="px-2.5 py-1 rounded-full bg-blue-50 text-blue-700 text-xs font-medium border border-blue-100">
                        {exp.Config?.Build?.BaseImage || "unknown"}
                      </span>
                    </td>
                    <td className="px-6 py-4">{exp.Config?.Trials || 1}</td>
                    <td className="px-6 py-4">{exp.Config?.Stages?.length || 0} stages</td>
                    <td className="px-6 py-4">{(exp.Baseline?.PowerWatts || 0).toFixed(2)} W</td>
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
