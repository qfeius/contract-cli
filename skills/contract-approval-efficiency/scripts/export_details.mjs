// Invoked by export_details.py in a temporary directory with a dependency link.
import fs from "node:fs/promises";
import { Workbook, SpreadsheetFile } from "@oai/artifact-tool";

async function main() {
  const [input, output, previewPath] = process.argv.slice(2);
  if (!input || !output) throw new Error("需要 payload.json 和输出路径");
  const payload = JSON.parse(await fs.readFile(input, "utf8"));
  const workbook = Workbook.create();
  const sheet = workbook.worksheets.add("审批效率明细");
  const headerRow = 7;
  const finalRow = headerRow + payload.rows.length;
  sheet.showGridLines = false;
  sheet.tabColor = "#234564";
  sheet.getRange(`A1:O${Math.max(finalRow, headerRow)}`).format = {
    font: { name: "Arial", size: 10, color: "#253746" },
    verticalAlignment: "center",
    rowHeight: 25,
  };
  const widths = [38, 27, 27, 25, 22, 19, 24, 24, 15, 14, 27, 27, 16, 19, 42];
  for (let column = 0; column < widths.length; column++) {
    sheet.getRangeByIndexes(0, column, Math.max(finalRow, headerRow), 1).format.columnWidth = widths[column];
  }
  sheet.getRange("A2").values = [["审批、盖章与归档明细"]];
  sheet.getRange("A2").format.font = { name: "Arial", size: 15, bold: true, color: "#18334D" };
  sheet.getRange("A2:O2").format.borders = { bottom: { style: "thin", color: "#A9B9C8" } };
  sheet.getRange("A3").values = [[payload.period]];
  sheet.getRange("A4").values = [[payload.scope]];
  sheet.getRange("A5").values = [[payload.source]];
  sheet.getRange("A6").values = [[payload.coverage]];
  sheet.getRange("A3:O6").format.font = { name: "Arial", size: 10, color: "#596C7D" };
  sheet.getRange(`A${headerRow}:O${headerRow}`).values = [payload.headers];
  if (payload.rows.length) {
    // Apply text formats before assigning identifier values so both the saved
    // workbook and the preview renderer preserve long numeric IDs verbatim.
    for (const column of ["B", "C", "K", "L"]) {
      sheet.getRange(`${column}8:${column}${finalRow}`).setNumberFormat("@");
    }
  }
  for (let start = 0; start < payload.rows.length; start += 5000) {
    const rows = payload.rows.slice(start, start + 5000);
    sheet.getRangeByIndexes(headerRow + start, 0, rows.length, payload.headers.length).values = rows;
  }
  if (payload.rows.length) {
    sheet.getRange(`G8:H${finalRow}`).setNumberFormat("yyyy-mm-dd hh:mm:ss");
    sheet.getRange(`I8:I${finalRow}`).setNumberFormat("#,##0.00");
    sheet.getRange(`I8:I${finalRow}`).format.horizontalAlignment = "right";
    sheet.getRange(`A8:F${finalRow}`).format.wrapText = true;
    sheet.getRange(`O8:O${finalRow}`).format.wrapText = true;
    sheet.getRange(`A8:O${finalRow}`).format.autofitRows();
  }
  const table = sheet.tables.add(`A${headerRow}:O${Math.max(finalRow, headerRow)}`, true, "ApprovalDetails");
  table.showFilterButton = true;
  sheet.getRange(`A${headerRow}:O${headerRow}`).format = {
    fill: "#234564", font: { name: "Arial", size: 10, bold: true, color: "#FFFFFF" },
    horizontalAlignment: "center", verticalAlignment: "center", rowHeight: 30,
    borders: { insideVertical: { style: "thin", color: "#FFFFFF" } },
  };
  sheet.freezePanes.freezeRows(headerRow);
  workbook.recalculate();
  const sampleEnd = Math.min(Math.max(finalRow, headerRow), headerRow + 3);
  await workbook.inspect({ kind: "table", range: `审批效率明细!A7:O${sampleEnd}`,
    include: "values,formulas", tableMaxRows: 4, tableMaxCols: 15, maxChars: 3000 });
  // The source rows are static audit records; formula presence is checked again
  // in the saved OOXML by the Python caller before publishing the result.
  await workbook.inspect({ kind: "match", searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#NUM!|#NULL!|#SPILL!|#CALC!",
    options: { useRegex: true, maxResults: 10 }, maxChars: 1000 });
  if (previewPath) {
    const image = await workbook.render({ sheetName: sheet.name,
      range: `A1:O${Math.min(Math.max(finalRow, headerRow), 13)}`, scale: 1.3, format: "png" });
    await fs.writeFile(previewPath, new Uint8Array(await image.arrayBuffer()));
  }
  const xlsx = await SpreadsheetFile.exportXlsx(workbook);
  await xlsx.save(output);
  process.stdout.write(JSON.stringify({ row_count: payload.rows.length }));
}

main().catch((error) => {
  process.stderr.write(`export_details: ${error.message}\n`);
  process.exitCode = 1;
});
