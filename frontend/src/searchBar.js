import { useState } from "react";
import InputGroup from "react-bootstrap/InputGroup";
import "react-datepicker/dist/react-datepicker.css";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Form from "react-bootstrap/Form";

export function SearchBar({ setLocation, setShowInvalidAlerts }) {
  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  return (
    <Container className="mt-3">
      <Row>
        <InputGroup className="mb-2">
          <Col className="col-auto">
            <Form.Control
              type="date"
              id={"search-bar-date"}
              value={date}
              onChange={(e) => setDate(e.target.value)}
            />
          </Col>
          <Col>
            <Form.Control
              type="text"
              placeholder="Los Angeles, CA, USA"
              id={"search-bar-location"}
              onChange={(e) => {
                setLocation(e.target.value);
                setShowInvalidAlerts(false);
              }}
            />
          </Col>
        </InputGroup>
      </Row>
    </Container>
  );
}
