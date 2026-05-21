import * as React from "react";
import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DndContext } from "@dnd-kit/core";
import { OperatorLibrary } from "./operator-library";
import "@/lib/i18n";
import i18n from "@/lib/i18n";

// Ensure i18n is initialised in English for deterministic test assertions
// against the operator name / description strings.
beforeAll(async () => {
  await i18n.changeLanguage("en");
});

function Wrapper({
  onClickAdd,
}: {
  onClickAdd?: (type: string) => void;
}) {
  return (
    <DndContext>
      <OperatorLibrary onClickAdd={onClickAdd as never} />
    </DndContext>
  );
}

describe("OperatorLibrary", () => {
  it("renders all 6 operator cards", () => {
    render(<Wrapper />);
    expect(
      screen.getByTestId("operator-library-card-filter"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-map"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-sort"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-dedupe"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-regex_rename"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-output"),
    ).toBeInTheDocument();
  });

  it("filters cards by search query (matches against translated name)", () => {
    render(<Wrapper />);
    const search = screen.getByTestId(
      "operator-library-search",
    ) as HTMLInputElement;

    fireEvent.change(search, { target: { value: "filter" } });

    expect(screen.getByTestId("operator-library-card-filter")).toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-library-card-sort"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-library-card-output"),
    ).not.toBeInTheDocument();
  });

  it("clears the search when the query is removed and re-renders all cards", () => {
    render(<Wrapper />);
    const search = screen.getByTestId(
      "operator-library-search",
    ) as HTMLInputElement;

    fireEvent.change(search, { target: { value: "filter" } });
    expect(
      screen.queryByTestId("operator-library-card-sort"),
    ).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: "" } });
    expect(screen.getByTestId("operator-library-card-sort")).toBeInTheDocument();
    expect(
      screen.getByTestId("operator-library-card-output"),
    ).toBeInTheDocument();
  });

  it("matches against the description text, not just the name", () => {
    render(<Wrapper />);
    const search = screen.getByTestId(
      "operator-library-search",
    ) as HTMLInputElement;

    // The dedupe card description references "duplicates"; the other
    // operator descriptions do not, so we expect only dedupe to survive.
    fireEvent.change(search, { target: { value: "duplicates" } });
    expect(screen.getByTestId("operator-library-card-dedupe")).toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-library-card-filter"),
    ).not.toBeInTheDocument();
  });

  it("matches against the on-the-wire operator id (e.g. regex_rename)", () => {
    render(<Wrapper />);
    const search = screen.getByTestId(
      "operator-library-search",
    ) as HTMLInputElement;
    fireEvent.change(search, { target: { value: "regex_rename" } });
    expect(
      screen.getByTestId("operator-library-card-regex_rename"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("operator-library-card-filter"),
    ).not.toBeInTheDocument();
  });

  it("invokes onClickAdd with the operator type when a card is clicked", () => {
    const onClickAdd = vi.fn();
    render(<Wrapper onClickAdd={onClickAdd} />);
    fireEvent.click(screen.getByTestId("operator-library-card-sort"));
    expect(onClickAdd).toHaveBeenCalledWith("sort");
  });
});
