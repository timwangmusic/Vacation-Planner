import React from "react"
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import Col from 'react-bootstrap/Col'

const Login = () => {
  return (
<Form>
    <Form.Row>
        <Form.Group as={Col} controlId="formGridFirstName">
            <Form.Label>First Name</Form.Label>
            <Form.Control type='first name' placeholder="First name" />
        </Form.Group>

        <Form.Group as={Col} controlId="formGridSecondName">
            <Form.Label>Last Name</Form.Label>
            <Form.Control type='last name' placeholder="Last name"/>
        </Form.Group>
    </Form.Row>

    <Form.Group controlId="formBasicEmail">
        <Form.Label>Email address</Form.Label>
        <Form.Control type="email" placeholder="Enter email" />
        <Form.Text className="text-muted">
            We'll never share your email with anyone else.
        </Form.Text>
  </Form.Group>

  <Form.Group controlId="formBasicPassword">
    <Form.Label>Password</Form.Label>
    <Form.Control type="password" placeholder="Password" />
    </Form.Group>
        <Form.Group controlId="formBasicCheckbox">
            <Form.Check type="checkbox" label="Check me out" />
    </Form.Group>

    <Button variant="primary" type="submit">
        Submit
    </Button>

</Form>
  );
}

export default Login
