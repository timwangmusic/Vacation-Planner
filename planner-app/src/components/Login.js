import React  from "react"
import Form from 'react-bootstrap/Form'
import Button from 'react-bootstrap/Button'
import Col from 'react-bootstrap/Col'
import useForm from './useForm.js'

// Using hooks instead of a Class and constructor


const Login = () => {

    function signup(){
        console.log(values)
    }

        const {
        values,
        handleChange,
        handleSubmit
    } = useForm(signup)

    return (
<Form noValidate onSubmit={handleSubmit}>
    <Form.Row>
        <Form.Group as={Col} controlId="formGridUserName">
            <Form.Label>UserName</Form.Label>
            <Form.Control
                autoFocus
                name={'username'}
                value={values.username}
                type='username'
                placeholder="username"
                onChange={handleChange}
            />
        </Form.Group>
    </Form.Row>

  <Form.Group controlId="formBasicPassword">
    <Form.Label>Password</Form.Label>
    <Form.Control
        autoFocus
        type="password"
        name={'password'}
        value={values.password}
        onChange={handleChange}
        placeholder="Password" />
    </Form.Group>
        <Form.Group controlId="formBasicCheckbox">
            <Form.Check type="checkbox" label="Check me out" />
    </Form.Group>

    <Button
        variant="primary"
        type="submit"
        value="Submit"
        >
        Login
    </Button>

</Form>
    );
}

export default Login
