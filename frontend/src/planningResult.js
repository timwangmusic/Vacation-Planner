import Table from "react-bootstrap/Table";

export function PlanningResults({ plans }) {
  if (plans != null) {
    return (
      <div>
        {plans.map((plan) => (
          <PlanningResult places={plan.places} key={plan.id} />
        ))}
      </div>
    );
  }
  return <div />;
}

function PlanningResult({ places }) {
  return (
    <Table striped bordered hover size="sm">
      <thead>
        <tr>
          <th style={{ width: "15%" }}>Time</th>
          <th style={{ width: "30%" }}>Name</th>
          <th>Address</th>
        </tr>
      </thead>
      <tbody>
        {places.map((place, index) => (
          <ResultRow
            key={index}
            start={place.start_time}
            end={place.end_time}
            name={place.place_name}
            address={place.address}
          />
        ))}
      </tbody>
    </Table>
  );
}

function ResultRow({ start, end, name, address }) {
  return (
    <tr>
      <td>{start + " - " + end}</td>
      <td>{name}</td>
      <td>{address}</td>
    </tr>
  );
}
