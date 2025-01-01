import Table from "react-bootstrap/Table";
import Form from "react-bootstrap/Form";
import Container from "react-bootstrap/Container";
import { useState } from "react";
import Button from "react-bootstrap/Button";
import Row from "react-bootstrap/Row";
import ButtonGroup from "react-bootstrap/ButtonGroup";
import { SearchBar } from "./searchBar";
import Spinner from "react-bootstrap/Spinner";
import { MyAlert } from "./alerts";
import axios from "axios";
import { planningRequest } from "./utils";
import { PaginatedItems } from "./paginatedItems";
import $ from "jquery";

const SelectKindCategoryName = "category";
const SelectKindTimeHourName = "hour";

export function RequestForm() {
  const [numRows, setNumRows] = useState(3);
  const [location, setLocation] = useState("");
  const [resultLoading, setResultLoading] = useState(false);
  const [showInvalidRequestAlert, setShowInvalidRequestAlert] = useState(false);
  const [showNoSolutionAlert, setshowNoSolutionAlert] = useState(false);
  const [plans, setPlans] = useState([]);

  const addRow = () => {
    setNumRows((numRows) => numRows + 1);
  };

  const removeRow = () => {
    setNumRows((numRows) => Math.max(1, numRows - 1));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setshowNoSolutionAlert(false);
    // reset previous results
    setPlans([]);
    setResultLoading(true);
    await postPlanningRequest();
    setResultLoading(false);
  };

  async function postPlanningRequest() {
    const date = $("#search-bar-date").val();
    const postReqData = planningRequest();
    if (postReqData.invalid) {
      setShowInvalidRequestAlert(true);
      return;
    }

    await axios
      .post(`/v1/customize?date=${date}`, JSON.stringify(postReqData))
      .then((response) => {
        switch (response.data?.status_code) {
          case 200:
            setPlans(response.data.travel_plans);
            return;
          case 404:
            setshowNoSolutionAlert(true);
            return;
          default:
            setshowNoSolutionAlert(true);
        }
      })
      .catch(console.error);
  }

  return (
    <Container className="mt-2">
      <Row>
        <SearchBar
          setLocation={setLocation}
          setShowInvalidAlerts={setShowInvalidRequestAlert}
        />
      </Row>
      <RequestTable rowCount={numRows} id={"request-table"} />
      <Row>
        <div className="d-flex justify-content-end">
          <ButtonGroup className="btn-group-sm mb-3">
            <Button
              className="me-2"
              variant="outline-primary"
              id="add-row"
              onClick={addRow}
            >
              Add
            </Button>
            <Button
              className="me-2"
              variant="outline-primary"
              id="remove-row"
              onClick={removeRow}
            >
              Remove
            </Button>
            <Button
              variant="outline-success"
              id="search"
              onClick={handleSubmit}
              disabled={location.length === 0 || resultLoading}
            >
              Search
            </Button>
          </ButtonGroup>
          <LoadingSpinner loading={resultLoading} />
        </div>
      </Row>
      <Row>
        <MyAlert
          message={
            "Please correct planning request form inputs and try search again"
          }
          show={showInvalidRequestAlert}
          variant={"danger"}
        ></MyAlert>
      </Row>
      <Row>
        <MyAlert
          message={
            "No valid result is found, please modify the search form and try again"
          }
          show={showNoSolutionAlert}
          variant={"info"}
        />
      </Row>
      <Row>
        <PaginatedItems items={plans} itemsPerPage={2} />
      </Row>
    </Container>
  );
}

// returns a Table representing the collection of slots in a planning request
function RequestTable({ rowCount, id }) {
  return (
    <Table striped bordered id={id}>
      <thead>
        <tr>
          <th>Category</th>
          <th>Start Hour</th>
          <th>End Hour</th>
        </tr>
      </thead>
      <tbody>
        {[...Array(rowCount).keys()].map((rowIdx) => (
          <RequestRow id={rowIdx} key={rowIdx} />
        ))}
      </tbody>
    </Table>
  );
}

// returns a Table.row allowing users to select a category and start/end hours of a day
function RequestRow({ id }) {
  return (
    <tr id={id}>
      <td>
        <SlotSelect kind={SelectKindCategoryName} displayName={"category"} />
      </td>
      <td>
        <SlotSelect kind={SelectKindTimeHourName} displayName={"start"} />
      </td>
      <td>
        <SlotSelect kind={SelectKindTimeHourName} displayName={"end"} />
      </td>
    </tr>
  );
}

// returns a Form.select based on select kind
function SlotSelect({ kind, displayName }) {
  switch (kind) {
    case SelectKindCategoryName:
      return (
        <Form.Select>
          <option>{displayName}</option>
          {["Visit", "Eatery"].map((c) => (
            <option key={c}>{c}</option>
          ))}
        </Form.Select>
      );
    case SelectKindTimeHourName:
      return (
        <Form.Select>
          <option>{displayName}</option>
          {[...Array(24).keys()].map((h) => (
            <option key={h}>{h}</option>
          ))}
        </Form.Select>
      );
    default:
      break;
  }

  return <Form.Select />;
}

function LoadingSpinner({ loading }) {
  if (loading) {
    return (
      <Spinner
        animation="border"
        size="sm"
        className="mx-2 mt-1"
        variant="info"
      ></Spinner>
    );
  }
  return null;
}
