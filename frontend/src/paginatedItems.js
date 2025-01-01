import { useState } from "react";
import { PlanningResults } from "./planningResult";
import ReactPaginate from "react-paginate";
import "./pagination.css";

export function PaginatedItems({ items, itemsPerPage }) {
  const [offset, setOffset] = useState(0);

  const endOffset = offset + itemsPerPage;
  const currentItems = items.slice(offset, endOffset);
  const pageCount = Math.ceil(items.length / itemsPerPage);

  const handleClick = (e) => {
    const newOffset = (e.selected * itemsPerPage) % items.length;
    setOffset(newOffset);
  };

  return (
    <>
      <PlanningResults plans={currentItems} />
      <ReactPaginate
        breakLabel="..."
        nextLabel="next >"
        onPageChange={handleClick}
        pageRangeDisplayed={5}
        pageCount={pageCount}
        previousLabel="< previous"
        renderOnZeroPageCount={null}
        containerClassName="pagination"
      />
    </>
  );
}
